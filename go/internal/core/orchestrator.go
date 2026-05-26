package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
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
	Cycle         int
	FinalVerdict  string
	PhasesRun     []Phase
	// RetroDecision is the failure-adapter's verdict on the retro branch,
	// populated only when retro ran. Format: "<action>: <reason>".
	RetroDecision string
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
	// gitHEAD returns the current git HEAD SHA. Called once at cycle
	// start and once before finalizing the verdict so the orchestrator
	// can detect whether anything got committed during the cycle (e.g.
	// when the build phase invokes `evolve ship --class manual` inline).
	// Errors are swallowed and treated as "no movement detected" — the
	// outcome calculator falls back to SKIPPED_UNKNOWN.
	gitHEAD func() (string, error)
}

// NewOrchestrator wires the orchestrator with its dependencies.
func NewOrchestrator(storage Storage, ledger Ledger, runners map[Phase]PhaseRunner) *Orchestrator {
	return &Orchestrator{
		storage: storage,
		ledger:  ledger,
		runners: runners,
		sm:      NewStateMachine(),
		now:     time.Now,
		gitHEAD: defaultGitHEAD,
	}
}

// archivePollutedWorkspace renames <workspace>/ to
// <workspace>.polluted-<UTCnano>/ when it exists and is non-empty.
// Returns nil for the empty-or-missing case (the cycle just runs in a
// fresh directory). Returns the underlying error only when stat/rename
// actually fails. Tests inject a deterministic clock via now.
func archivePollutedWorkspace(workspace string, now func() time.Time) error {
	info, err := os.Stat(workspace)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat workspace: %w", err)
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(workspace)
	if err != nil {
		return fmt.Errorf("readdir workspace: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}
	stamp := now().UTC().Format("20060102T150405.000000000")
	archived := workspace + ".polluted-" + stamp
	if err := os.Rename(workspace, archived); err != nil {
		return fmt.Errorf("rename to %s: %w", archived, err)
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] archived polluted workspace: %s -> %s (%d files)\n",
		workspace, archived, len(entries))
	return nil
}

// defaultGitHEAD runs `git rev-parse HEAD` in cwd.
// Returns empty string on error AND emits a one-line WARN to stderr so
// operators see the degraded-mode signal that yields SKIPPED_UNKNOWN.
// finalizeOutcome treats equal strings as no movement.
func defaultGitHEAD() (string, error) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN git HEAD probe failed (cycle outcome labels degraded): %v\n", err)
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

// finalizeOutcome translates SKIPPED into a more specific CycleOutcome label
// using HEAD movement and retro text as signals. PASS/FAIL/WARN pass through.
func (o *Orchestrator) finalizeOutcome(lastPhaseVerdict, retroDecision, preHEAD, postHEAD string) string {
	if lastPhaseVerdict != VerdictSKIPPED {
		return lastPhaseVerdict
	}
	// HEAD moved → something shipped inline (build calling `evolve ship --class manual`).
	if preHEAD != "" && postHEAD != "" && preHEAD != postHEAD {
		return CycleOutcomeShippedViaBuild
	}
	if strings.Contains(retroDecision, "would-have-blocked") {
		return CycleOutcomeSkippedAuditAdvisory
	}
	return CycleOutcomeSkippedUnknown
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
	// IntentRequired is the gate for the start→intent vs start→scout
	// edge. Source priority: explicit Context["intent_required"]=="true"
	// from the caller > env EVOLVE_REQUIRE_INTENT=="1" > false. This
	// mirrors the bash dispatcher's check at run-cycle.sh:build_context.
	intentRequired := req.Context["intent_required"] == "true" ||
		req.Env["EVOLVE_REQUIRE_INTENT"] == "1"
	cs := CycleState{
		CycleID:        cycle,
		Phase:          string(PhaseStart),
		StartedAt:      startedAt,
		PhaseStartedAt: startedAt,
		WorkspacePath:  fmt.Sprintf("%s/.evolve/runs/cycle-%d", req.ProjectRoot, cycle),
		IntentRequired: intentRequired,
	}
	// Guard against workspace pollution from a prior killed attempt at
	// the same cycle number. If `<workspace>/` exists and has files,
	// rename to `<workspace>.polluted-<UTCnano>/` BEFORE any phase runs.
	// Without this, leftover scout-report.md / build-report.md from the
	// killed attempt cause Scout to short-circuit (read pre-existing
	// artifacts in seconds instead of redoing discovery) and steer
	// downstream phases via the OLD task selection.
	// Source incident: cycle-108 meta-loop attempts 1-4 (2026-05-26).
	// Opt-out via EVOLVE_DISABLE_WORKSPACE_GUARD=1 — used by tests that
	// pre-seed workspace files to simulate phase state.
	if req.Env["EVOLVE_DISABLE_WORKSPACE_GUARD"] != "1" && os.Getenv("EVOLVE_DISABLE_WORKSPACE_GUARD") != "1" {
		if err := archivePollutedWorkspace(cs.WorkspacePath, o.now); err != nil {
			// Best-effort: WARN but don't block the cycle; the failure
			// mode it prevents is bad-data steering, not safety.
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN workspace archive failed: %v\n", err)
		}
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

	// Capture HEAD before any phase so finalizeOutcome can detect mid-cycle commits.
	preCycleHEAD, _ := o.gitHEAD()

	result := CycleResult{Cycle: cycle, FinalVerdict: VerdictPASS}
	current := PhaseStart
	lastVerdict := VerdictPASS
	// scheduledNext, when non-empty, overrides the state machine for
	// the next iteration. Set by the retro branch to inject the
	// failure-adapter's decision.
	var scheduledNext Phase

	// Bounded loop guards against any transition-table cycle bug.
	for safety := 0; safety < 32; safety++ {
		var next Phase
		switch {
		case scheduledNext != "":
			next = scheduledNext
			scheduledNext = ""
		case current == PhaseStart:
			// First edge is gated by intent_required, not by verdict.
			next = o.sm.NextFromStart(cs.IntentRequired)
		default:
			n, err := o.sm.Next(current, lastVerdict)
			if err != nil {
				return result, fmt.Errorf("transition from %s: %w", current, err)
			}
			next = n
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

		// Retro is the one phase whose successor isn't verdict-driven;
		// the failure-adapter consults cycle history (state.FailedAt) and
		// the retro verdict to pick {ship | tdd | end}. Set scheduledNext
		// so the next loop iteration runs the chosen phase.
		if current == PhaseRetro {
			branch, extraEnv, reason := o.decideAfterRetro(resp.Verdict, state.FailedAt)
			for k, v := range extraEnv {
				envSnap[k] = v
			}
			result.RetroDecision = reason
			if branch == PhaseEnd {
				break
			}
			if !o.sm.CanTransition(PhaseRetro, branch) {
				return result, fmt.Errorf("retro→%s not allowed by state machine", branch)
			}
			scheduledNext = branch
		}
	}

	postCycleHEAD, _ := o.gitHEAD()
	result.FinalVerdict = o.finalizeOutcome(result.FinalVerdict, result.RetroDecision, preCycleHEAD, postCycleHEAD)

	state.LastCycleNumber = cycle
	if err := o.storage.WriteState(ctx, state); err != nil {
		return result, fmt.Errorf("write state: %w", err)
	}
	return result, nil
}

// decideAfterRetro consults the failure-adapter over cycle history
// (state.failedApproaches) to pick the post-retro branch.
//
// Mapping (retro verdict × failureadapter action → next phase):
//   - retro PASS               → ship   (retrospective recovered the cycle)
//   - retro FAIL/WARN + BLOCK-* → end    (cycle history forbids further work)
//   - retro FAIL/WARN + RETRY  → tdd    (retry from earlier phase w/ fallback env)
//   - retro FAIL/WARN + PROCEED → end   (no recovery, no block — exit cleanly)
//
// Returned reason is "<action>: <failureadapter reason>" for the
// CycleResult.RetroDecision audit field.
func (o *Orchestrator) decideAfterRetro(retroVerdict string, history []FailedRecord) (next Phase, extraEnv map[string]string, reason string) {
	// retro PASS → ship; no failureadapter consultation.
	if retroVerdict == VerdictPASS {
		return PhaseShip, nil, "retro-recovered: ship"
	}
	entries := entriesFromRecords(history)
	dec := failureadapter.Decide(entries, failureadapter.Options{Now: o.now()})
	switch dec.Action {
	case failureadapter.ActionRetryWithFallback:
		return PhaseTDD, dec.SetEnv, "retry-with-fallback: " + dec.Reason
	case failureadapter.ActionBlockCode, failureadapter.ActionBlockOperatorAction:
		return PhaseEnd, nil, string(dec.Action) + ": " + dec.Reason
	default: // ActionProceed
		return PhaseEnd, dec.SetEnv, "proceed: " + dec.Reason
	}
}

// entriesFromRecords converts FailedRecord values into failureadapter.Entry.
// Inlined here (rather than exposed from failureadapter) to avoid a
// circular import between core and failureadapter.
func entriesFromRecords(records []FailedRecord) []failureadapter.Entry {
	out := make([]failureadapter.Entry, len(records))
	for i, r := range records {
		out[i] = failureadapter.Entry{
			TS:                r.TS,
			Cycle:             r.Cycle,
			Verdict:           r.Verdict,
			Classification:    failureadapter.Classification(r.Classification),
			RecordedAt:        r.RecordedAt,
			ExpiresAt:         r.ExpiresAt,
			AuditReportPath:   r.AuditReportPath,
			AuditReportSHA256: r.AuditReportSHA256,
			GitHead:           r.GitHead,
			TreeStateSHA:      r.TreeStateSHA,
			Defects:           r.Defects,
			Retrospected:      r.Retrospected,
			Summary:           r.Summary,
		}
	}
	return out
}
