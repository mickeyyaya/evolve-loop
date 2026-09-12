// resume_parity_recording_test.go — resume-path parity regressions.
//
// The fresh cycle path records a terminal outcome at every way a cycle can
// end. The resume path, built as a parallel implementation, does not: three
// of its exits return a bare error where the fresh path first records the
// outcome and feeds failure-learning. The consequence is named in
// recordChokepointEscape's own doc comment — an unrecorded terminal exit
// classifies FAILED_UNEXPLAINED, "the alarm bucket (the cycle-492 escape)" —
// so a resumed cycle that dies this way pages an operator with no diagnosable
// reason, which is exactly the failure mode the fresh path was fixed for.
//
// Each test below mirrors an existing fresh-path test against
// RunCycleFromPhase. The compatibility table in
// docs/reports/2026-09-11-large-component-decomposition-plan.md lists the
// fresh/resume differences that ARE intentional (parallel evaluation,
// remediation, the debugger override, legacy contract versions); none of
// these three is on it, and the resume code documents the debugger omission
// inline while saying nothing about these.
package core_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclehealth"
)

// TestRunCycleFromPhase_TransitionCycleGuard_RecordsChokepointEscape is the
// resume mirror of TestRunCycle_TransitionCycleGuard_RecordsChokepointEscape.
// Same contract, same C1 invariant: exhausting the bounded dispatch loop
// without reaching PhaseEnd must record an explicit terminal abort so the
// cycle classifies FAILED_EXPLAINED rather than landing in the
// FAILED_UNEXPLAINED alarm bucket.
func TestRunCycleFromPhase_TransitionCycleGuard_RecordsChokepointEscape(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	// Resume reads its workspace from the persisted cycle state rather than
	// provisioning one, so the fixture must supply it the way a real
	// interrupted cycle would have.
	ws := filepath.Join(projectRoot, ".evolve", "runs", "cycle-640")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	st := &recStorage{
		state: core.State{LastCycleNumber: 640},
		cs:    core.CycleState{CycleID: 640, WorkspacePath: ws},
	}
	// Cap at 2, exactly as the fresh test does: the spine needs far more than
	// two dispatches to reach PhaseEnd, so the loop exits via the iteration
	// bound — the transition-cycle escape path.
	orch := core.NewOrchestrator(st, &fakeLedger{}, newRunners(nil),
		core.WithMaxPhaseIterations(2))

	_, err := orch.RunCycleFromPhase(context.Background(),
		core.CycleRequest{ProjectRoot: projectRoot, GoalHash: "test-goal"},
		&core.ResumePoint{Phase: string(core.PhaseScout), CycleID: 640})
	if err == nil {
		t.Fatal("want the iteration bound to surface an error")
	}
	if !strings.Contains(err.Error(), "iteration limit") {
		t.Fatalf("err = %v, want the iteration-limit escape", err)
	}

	ws = findCycleWorkspace(t, projectRoot)
	outcome, detail := cyclehealth.ClassifyOutcome(ws)
	if outcome == cyclehealth.OutcomeFailedUnexplained {
		t.Fatalf("resumed cycle classified FAILED_UNEXPLAINED — the C1 chokepoint escaped on the resume path exactly as it did on the fresh path before the cycle-492 fix; detail=%q", detail)
	}
	if outcome != cyclehealth.OutcomeFailedExplained {
		t.Errorf("outcome = %q, want FAILED_EXPLAINED (%q)", outcome, cyclehealth.OutcomeFailedExplained)
	}
	// Same assertion as the fresh twin: "some reason exists" is a weaker
	// contract than "the escape named the stalled phase", and diagnosability
	// is the entire point of the fix.
	if !strings.Contains(detail, "transition-table cycle") {
		t.Errorf("abort detail = %q, want it to name the transition-table cycle (the diagnosable reason)", detail)
	}
}
