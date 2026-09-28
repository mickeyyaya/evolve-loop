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
