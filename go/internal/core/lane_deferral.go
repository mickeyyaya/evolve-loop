package core

import (
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type LaneDeferral struct {
	Cycle int
	Cause error
}

func (d *LaneDeferral) Error() string {
	return fmt.Sprintf("fleet lane deferred (cycle %d): worktree provisioning failed: %v", d.Cycle, d.Cause)
}

func (d *LaneDeferral) Unwrap() error { return d.Cause }

func (o *Orchestrator) reprovisionWorktree(projectRoot string, cycle int, first error) (string, error) {
	fmt.Fprintf(os.Stderr, "[orchestrator] WARN worktree provisioning failed, re-provisioning once: %v\n", first)
	return o.worktree.Create(projectRoot, cycle)
}

func (o *Orchestrator) deferLaneWithoutWorktree(cs CycleState, cause error) error {
	reason := cyclestate.CycleTerminationLaneWorktreeDeferred + ": " + cause.Error()
	cr := &cycleRun{o: o, cs: cs, cycle: cs.CycleID}
	o.recordHostEnding(&cr.phaseTimings, phaseOutcomeFrom(PhaseStart,
		PhaseResponse{Phase: string(PhaseStart), Verdict: VerdictSKIPPED}, 0, reason, cs.StartedAt))
	if err := os.MkdirAll(cs.WorkspacePath, 0o755); err != nil {
		return fmt.Errorf("fleet lane deferral for cycle %d not recorded (%v), so the breaker could not count it; worktree provisioning failed: %w", cs.CycleID, err, cause)
	}
	cr.flushPhaseTimings()
	o.signals.Emit(signalcenter.Event{
		Cycle: cs.CycleID, RunID: cs.RunID, Module: signalcenter.ModuleOrchestrator, Origin: "Orchestrator.deferLaneWithoutWorktree",
		Kind: signalcenter.KindCycleSealed, Severity: signalcenter.SeverityWarn, Code: CodeLaneDeferred,
		Reason: "fleet lane deferred before any phase dispatch: worktree provisioning failed: " + cause.Error(),
		Fields: map[string]string{"step": "worktree", "cause": cause.Error()},
	})
	return &LaneDeferral{Cycle: cs.CycleID, Cause: cause}
}

func (o *Orchestrator) recordProvisioningFailure(cycle int, projectRoot, workspace string, cause error) {
	fmt.Fprintf(os.Stderr, "[orchestrator] WARN worktree provisioning failed (source phases will be blocked): %v\n", cause)
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN create workspace for provisioning failure: %v\n", err)
		return
	}
	o.ensureFailureDigest(cycle, projectRoot, workspace, "worktree", fmt.Sprintf("worktree provisioning failed: %v", cause))
}
