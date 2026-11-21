package main

import (
	"bytes"
	"context"
	"slices"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestProductionPoolPlanFn_AReleasedMainRedStopRelaunchesTheParkedLeadThePoolAlreadyLaunched(t *testing.T) {
	t.Parallel()
	p := newParkedPool(t)
	p.park(2242, core.MainRedReason, "parked-a")
	p.hold(p.repo.Git("rev-parse", "HEAD"))
	p.release()
	var stderr bytes.Buffer
	plan := poolPlanner{cfg: loopConfig{ProjectRoot: p.repo.Dir, EvolveDir: p.evolveDir}, storage: &erroringReadStorage{}, stderr: &stderr}.plan(2)

	var mu sync.Mutex
	var launched []string
	launch := func(_ context.Context, spec fleet.CycleSpec) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		launched = append(launched, spec.Scope[0])
		return 0, nil
	}
	var once sync.Once
	var planned []fleet.Todo
	resupply := func([]fleet.Todo) ([]fleet.Todo, error) {
		var todos []fleet.Todo
		var err error
		once.Do(func() {
			todos, err = plan(context.Background(), 0)
			planned = todos
		})
		return todos, err
	}
	b, _ := iscsCoordinator(t.TempDir(), t.TempDir(), nil)
	fc := policy.FleetConfig{Count: 4, PlanSource: "triage", Orchestrator: enforceOrchestrator}

	b.orchestratedRun(fc, launch, 2)(context.Background(), []fleet.Todo{{ID: "parked-a"}}, resupply)

	if len(planned) == 0 || planned[0].ID != "parked-a" || !planned[0].Resume {
		t.Errorf("planned = %+v stderr %q, want the released parked scope first and marked to resume", planned, stderr.String())
	}
	mu.Lock()
	defer mu.Unlock()
	if n := slices.Index(launched, "parked-a"); n < 0 || slices.Index(launched[n+1:], "parked-a") < 0 {
		t.Errorf("launches = %v, want parked-a launched a second time after its lane exited", launched)
	}
}
