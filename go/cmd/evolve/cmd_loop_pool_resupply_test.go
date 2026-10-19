package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestPlanAndRunPool_TheResupplyPlansTheSameWaveDeeperByTheHeldTodos(t *testing.T) {
	t.Parallel()
	fc := policy.FleetConfig{Count: 2, PlanSource: "triage", Orchestrator: policy.OrchestratorConfig{Stage: "enforce", ScanK: 5}}
	type call struct{ depth, wave int }
	var calls []call
	plan := func(depth int) poolPlanFn {
		return func(_ context.Context, wave int) ([]fleet.Todo, error) {
			calls = append(calls, call{depth, wave})
			return []fleet.Todo{{ID: "ready"}}, nil
		}
	}
	run := func(_ context.Context, backlog []fleet.Todo, resupply poolSupplyFn) []fleet.Result {
		if _, err := resupply([]fleet.Todo{{ID: "a"}, {ID: "b"}, {ID: "c"}}); err != nil {
			t.Fatalf("resupply error = %v", err)
		}
		return make([]fleet.Result, len(backlog)+1)
	}
	ran, results, err := planAndRunPool(context.Background(), fc, func() error { return nil }, plan, run, 7)
	if err != nil || !ran || len(results) != 2 {
		t.Fatalf("ran=%v results=%d err=%v, want a run whose results cover the resupplied todo", ran, len(results), err)
	}
	if want := []call{{5, 7}, {8, 7}}; !reflect.DeepEqual(calls, want) {
		t.Errorf("plan calls = %v, want %v: the resupply plans wave 7 at the supply depth plus the 3 held todos", calls, want)
	}
}

func TestOrchestratorResupply_PassesThePlanAndWarnsOnAPlanError(t *testing.T) {
	t.Parallel()
	b, console := iscsCoordinator(t.TempDir(), t.TempDir(), nil)
	ok := b.orchestratorResupply(func([]fleet.Todo) ([]fleet.Todo, error) { return []fleet.Todo{{ID: "fresh"}}, nil }, 3)
	if got := ok(nil); len(got) != 1 || got[0].ID != "fresh" {
		t.Errorf("resupply = %v, want the planned fresh todo", got)
	}
	failed := b.orchestratorResupply(func([]fleet.Todo) ([]fleet.Todo, error) { return nil, errors.New("inbox unreadable") }, 3)
	if got := failed(nil); got != nil {
		t.Errorf("resupply after a plan error = %v, want none", got)
	}
	if !strings.Contains(console.String(), "[loop] WARN: orchestrator: pool 3 resupply: inbox unreadable") {
		t.Errorf("console = %q, want a WARN that names the plan error", console.String())
	}
}

func TestOrchestratedRun_TheResupplyFeedsTheFreedSlot(t *testing.T) {
	t.Parallel()
	b, _ := iscsCoordinator(t.TempDir(), t.TempDir(), nil)
	fc := policy.FleetConfig{Count: 1, PlanSource: "triage", Orchestrator: enforceOrchestrator}
	var launched []string
	launch := func(_ context.Context, spec fleet.CycleSpec) (int, error) {
		launched = append(launched, spec.Scope[0])
		return 0, nil
	}
	resupply := func([]fleet.Todo) ([]fleet.Todo, error) { return []fleet.Todo{{ID: "fresh"}}, nil }
	res := b.orchestratedRun(fc, launch, 2)(context.Background(), []fleet.Todo{{ID: "first"}}, resupply)
	if !reflect.DeepEqual(launched, []string{"first", "fresh"}) || len(res) != 2 {
		t.Errorf("launched = %v results = %d, want first then the resupplied fresh", launched, len(res))
	}
}

func TestRollingPool_KeepsTheFixedBacklogAndNeverAsksForResupply(t *testing.T) {
	t.Parallel()
	fc := policy.FleetConfig{Count: 2, PlanSource: "triage", Scheduling: "pool"}
	launch := func(context.Context, fleet.CycleSpec) (int, error) { return 0, nil }
	resupply := func([]fleet.Todo) ([]fleet.Todo, error) {
		t.Fatal("the rolling pool asked for a resupply; it has no launch cap, so its backlog stays fixed")
		return nil, nil
	}
	if res := rollingPool(fc, launch)(context.Background(), []fleet.Todo{{ID: "A"}}, resupply); len(res) != 1 {
		t.Errorf("results = %d, want 1", len(res))
	}
}
