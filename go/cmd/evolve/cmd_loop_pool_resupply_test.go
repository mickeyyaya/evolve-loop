package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestPlanAndRunPool_TheResupplyPlansTheSameWaveDeeperByTheHeldTodos(t *testing.T) {
	t.Parallel()
	fc := policy.FleetConfig{Count: 2, PlanSource: "triage", Orchestrator: policy.OrchestratorConfig{Stage: "enforce", ScanK: 5}}
	plan := func(depth int) poolPlanFn {
		return func(_ context.Context, wave int) ([]fleet.Todo, error) {
			return []fleet.Todo{{ID: fmt.Sprintf("depth%d-wave%d", depth, wave)}}, nil
		}
	}
	var firstBacklog, resupplied []fleet.Todo
	run := func(_ context.Context, backlog []fleet.Todo, resupply poolSupplyFn) []fleet.Result {
		firstBacklog = backlog
		fresh, err := resupply([]fleet.Todo{{ID: "a"}, {ID: "b"}, {ID: "c"}})
		if err != nil {
			t.Fatalf("resupply error = %v", err)
		}
		resupplied = fresh
		return make([]fleet.Result, len(backlog)+1)
	}
	ran, results, err := planAndRunPool(context.Background(), fc, func() error { return nil }, plan, run, 7)
	if err != nil || !ran || len(results) != 2 {
		t.Fatalf("ran=%v results=%d err=%v, want a run whose results cover the resupplied todo", ran, len(results), err)
	}
	if len(firstBacklog) != 1 || firstBacklog[0].ID != "depth5-wave7" {
		t.Errorf("first backlog = %v, want wave 7 planned at the supply depth 5", firstBacklog)
	}
	if len(resupplied) != 1 || resupplied[0].ID != "depth8-wave7" {
		t.Errorf("resupplied = %v, want wave 7 planned at the supply depth plus the 3 held todos", resupplied)
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
	if res := rollingPool(fc, launch, "", poolControl{})(context.Background(), []fleet.Todo{{ID: "A"}}, resupply); len(res) != 1 {
		t.Errorf("results = %d, want 1", len(res))
	}
}

func TestRollingPool_TellsEachLaneWhereTheLiveWidthLives(t *testing.T) {
	t.Parallel()
	fc := policy.FleetConfig{Count: 2, PlanSource: "triage", Scheduling: "pool"}
	path := filepath.Join(t.TempDir(), lanebudget.LiveWidthFile)
	var env map[string]string
	launch := func(_ context.Context, spec fleet.CycleSpec) (int, error) {
		env = spec.Env
		return 0, nil
	}
	rollingPool(fc, launch, path, poolControl{})(context.Background(), []fleet.Todo{{ID: "A"}}, nil)
	if env[ipcenv.FleetLiveWidthKey] != path || env[ipcenv.FleetWidthKey] != "2" {
		t.Errorf("lane env = %v, want %s=%s beside the spawn width", env, ipcenv.FleetLiveWidthKey, path)
	}
	var rec struct{ Width int }
	raw, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(raw, &rec) != nil || rec.Width != 1 {
		t.Errorf("live width file = %q, %v, want width 1 after the lane ends", raw, err)
	}
}

func TestLiveWidthFile_LivesUnderThePlaneEvolveDirectory(t *testing.T) {
	t.Parallel()
	if got, want := liveWidthFile("/plane/.evolve"), filepath.Join("/plane/.evolve", lanebudget.LiveWidthFile); got != want {
		t.Errorf("liveWidthFile = %q, want %q", got, want)
	}
}

func TestDispatchPool_TheProductionRunnerPassesTheLiveWidthFileToEachLane(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	b := &loopBatchCoordinator{ctx: context.Background(), cfg: loopConfig{EvolveDir: evolveDir}}
	fc := policy.FleetConfig{Count: 2, PlanSource: "triage", Scheduling: "pool"}
	var env map[string]string
	launch := func(_ context.Context, spec fleet.CycleSpec) (int, error) {
		env = spec.Env
		return 0, nil
	}
	run, mode := b.poolRunner(fc, launch, 1)
	run(context.Background(), []fleet.Todo{{ID: "A"}}, nil)
	if want := filepath.Join(evolveDir, lanebudget.LiveWidthFile); mode != "rolling" || env[ipcenv.FleetLiveWidthKey] != want {
		t.Errorf("mode = %q lane env %s = %q, want rolling and %q", mode, ipcenv.FleetLiveWidthKey, env[ipcenv.FleetLiveWidthKey], want)
	}
}
