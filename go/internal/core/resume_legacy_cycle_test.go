package core_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type cycleRecordingReviewer struct{ cycles []int }

func (r *cycleRecordingReviewer) Review(_ context.Context, in core.ReviewInput) core.ReviewResult {
	r.cycles = append(r.cycles, in.Cycle)
	return core.ReviewResult{Approve: true}
}

func TestResumeBootstrap_LegacyCheckpointReviewsUnderTheResumedCycle(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	ws := filepath.Join(projectRoot, ".evolve", "runs", "cycle-640")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	st := &recStorage{state: core.State{LastCycleNumber: 640}, cs: core.CycleState{WorkspacePath: ws}}
	rec := &cycleRecordingReviewer{}
	orch := core.NewOrchestrator(st, &fakeLedger{}, newRunners(nil), core.WithMaxPhaseIterations(2), core.WithReviewer(rec))
	_, _ = orch.RunCycleFromPhase(context.Background(),
		core.CycleRequest{ProjectRoot: projectRoot, GoalHash: "test-goal"},
		&core.ResumePoint{Phase: string(core.PhaseScout), CycleID: 640})
	if len(rec.cycles) == 0 {
		t.Fatal("the resumed phase was never reviewed")
	}
	for _, cycle := range rec.cycles {
		if cycle != 640 {
			t.Fatalf("a legacy checkpoint with no cycle_id must be reviewed under the resumed cycle 640, as the floor reviews a fresh one; reviewed cycles = %v", rec.cycles)
		}
	}
	if got := st.cs.CycleID; got != 640 {
		t.Fatalf("the resumed cycle state carries cycle %d, want the resumed cycle 640", got)
	}
}
