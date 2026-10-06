package campaign

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func TestRunWaves_ADeferredCycleIsRetriedNotCompleted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prog.json")
	calls := 0
	deferOnce := func(_ context.Context, w []fleet.CycleSpec) []fleet.Result {
		calls++
		res := make([]fleet.Result, len(w))
		for i := range w {
			res[i] = fleet.Result{Index: i}
			if calls == 1 {
				res[i].ExitCode = fleet.ExitDeferred
			}
		}
		return res
	}

	if err := RunWaves(context.Background(), [][]fleet.CycleSpec{mkwave("a")}, deferOnce, RunOptions{ProgressPath: path, PlanSHA: "P", MaxRetries: 1}); err != nil {
		t.Fatalf("RunWaves: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2: a deferred cycle released its item unworked, so the campaign must run it again", calls)
	}
}
