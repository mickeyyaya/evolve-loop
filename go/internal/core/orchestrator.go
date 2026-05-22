package core

import (
	"context"
	"fmt"
	"time"
)

// CycleRequest is the operator-facing input to RunCycle.
type CycleRequest struct {
	ProjectRoot string
	GoalHash    string
	Budget      BudgetEnvelope
	// Env is propagated to every PhaseRequest.Env that runs in this
	// cycle. Phases consult it for CLI/model selection
	// (EVOLVE_CLI, EVOLVE_<PHASE>_MODEL, …). The orchestrator copies the
	// map so post-RunCycle operator mutation does not affect in-flight
	// or completed runs.
	Env map[string]string
	// Context seeds the PhaseRequest.Context every phase receives. Ship
	// requires Context["commit_message"]; Scout reads
	// Context["strategy"]. Copied like Env.
	Context map[string]string
}

// CycleResult summarises what RunCycle did.
type CycleResult struct {
	Cycle        int
	FinalVerdict string
	PhasesRun    []Phase
}

// Orchestrator drives one cycle through the state machine, calling a
// PhaseRunner per phase and appending ledger entries. It is pure: all
// I/O is delegated to the injected Storage and Ledger ports.
//
// This is the Phase 1 skeleton — guards, observer, budget enforcement
// land in Phase 2.
type Orchestrator struct {
	storage Storage
	ledger  Ledger
	runners map[Phase]PhaseRunner
	sm      *StateMachine
	now     func() time.Time
}

// NewOrchestrator wires the orchestrator with its dependencies.
func NewOrchestrator(storage Storage, ledger Ledger, runners map[Phase]PhaseRunner) *Orchestrator {
	return &Orchestrator{
		storage: storage,
		ledger:  ledger,
		runners: runners,
		sm:      NewStateMachine(),
		now:     time.Now,
	}
}

// RunCycle drives one cycle from PhaseStart to PhaseEnd, returning a
// summary of what ran. The lock is acquired up front and released on
// every exit path. State is updated incrementally so a crash leaves an
// inspectable trail in .evolve/.
func (o *Orchestrator) RunCycle(ctx context.Context, req CycleRequest) (CycleResult, error) {
	release, err := o.storage.AcquireLock(ctx)
	if err != nil {
		return CycleResult{}, fmt.Errorf("acquire lock: %w", err)
	}
	defer func() { _ = release() }()

	state, err := o.storage.ReadState(ctx)
	if err != nil {
		return CycleResult{}, fmt.Errorf("read state: %w", err)
	}
	cycle := state.LastCycleNumber + 1

	startedAt := o.now().UTC().Format(time.RFC3339)
	cs := CycleState{
		CycleID:        cycle,
		Phase:          string(PhaseStart),
		StartedAt:      startedAt,
		PhaseStartedAt: startedAt,
		WorkspacePath:  fmt.Sprintf("%s/.evolve/runs/cycle-%d", req.ProjectRoot, cycle),
	}
	if err := o.storage.WriteCycleState(ctx, cs); err != nil {
		return CycleResult{}, fmt.Errorf("init cycle-state: %w", err)
	}

	// One snapshot per cycle — operator mutation post-call must not
	// retroactively change what phases saw.
	envSnap := make(map[string]string, len(req.Env))
	for k, v := range req.Env {
		envSnap[k] = v
	}
	ctxSnap := make(map[string]string, len(req.Context))
	for k, v := range req.Context {
		ctxSnap[k] = v
	}

	result := CycleResult{Cycle: cycle, FinalVerdict: VerdictPASS}
	current := PhaseStart
	lastVerdict := VerdictPASS

	// Bounded loop guards against any transition-table cycle bug.
	for safety := 0; safety < 32; safety++ {
		next, err := o.sm.Next(current, lastVerdict)
		if err != nil {
			return result, fmt.Errorf("transition from %s: %w", current, err)
		}
		if next == PhaseEnd {
			break
		}

		runner, ok := o.runners[next]
		if !ok {
			return result, fmt.Errorf("%w: no runner registered for phase %s", ErrPhaseInvalid, next)
		}

		phaseStarted := o.now().UTC()
		cs.Phase = string(next)
		cs.PhaseStartedAt = phaseStarted.Format(time.RFC3339)
		cs.ActiveAgent = string(next)
		if err := o.storage.WriteCycleState(ctx, cs); err != nil {
			return result, fmt.Errorf("write cycle-state pre-%s: %w", next, err)
		}

		resp, err := runner.Run(ctx, PhaseRequest{
			Cycle:         cycle,
			ProjectRoot:   req.ProjectRoot,
			Workspace:     cs.WorkspacePath,
			GoalHash:      req.GoalHash,
			Budget:        req.Budget,
			PreviousPhase: string(current),
			Env:           envSnap,
			Context:       ctxSnap,
		})
		if err != nil {
			return result, fmt.Errorf("phase %s: %w", next, err)
		}
		if !IsVerdict(resp.Verdict) {
			return result, fmt.Errorf("phase %s returned non-canonical verdict %q", next, resp.Verdict)
		}

		if err := o.ledger.Append(ctx, LedgerEntry{
			TS:       o.now().UTC().Format(time.RFC3339),
			Cycle:    cycle,
			Role:     string(next),
			Kind:     "phase",
			ExitCode: 0,
		}); err != nil {
			return result, fmt.Errorf("ledger append for %s: %w", next, err)
		}

		cs.CompletedPhases = append(cs.CompletedPhases, string(next))
		if err := o.storage.WriteCycleState(ctx, cs); err != nil {
			return result, fmt.Errorf("write cycle-state post-%s: %w", next, err)
		}

		result.PhasesRun = append(result.PhasesRun, next)
		result.FinalVerdict = resp.Verdict
		current = next
		lastVerdict = resp.Verdict

		// Retro can branch to either ship (recovered), tdd (retry), or
		// end (block). The Phase 1 skeleton defaults to ending after
		// retro; Phase 2 wires the failure-adapter for the real branch.
		if current == PhaseRetro {
			break
		}
	}

	state.LastCycleNumber = cycle
	if err := o.storage.WriteState(ctx, state); err != nil {
		return result, fmt.Errorf("write state: %w", err)
	}
	return result, nil
}
