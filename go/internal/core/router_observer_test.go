package core

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

type routerWatchObserver struct {
	mu       sync.Mutex
	active   int
	starts   []string
	requests []PhaseRequest
	cancels  int
}

func (r *routerWatchObserver) Start(_ context.Context, phase string, req PhaseRequest) func() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active++
	r.starts = append(r.starts, phase)
	r.requests = append(r.requests, req)
	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.active--
		r.cancels++
	}
}

func (r *routerWatchObserver) watching() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.active
}

type observedPlanner struct {
	obs             *routerWatchObserver
	planWatched     bool
	rePlanWatched   bool
	plan            *router.PhasePlan
	planCalls       int
	rePlanCallCount int
}

func (p *observedPlanner) Plan(router.RouteInput) (*router.PhasePlan, error) {
	p.planCalls++
	p.planWatched = p.obs.watching() == 1
	return p.plan, nil
}

func (p *observedPlanner) RePlan(router.RouteInput) (*router.PhasePlan, error) {
	p.rePlanCallCount++
	p.rePlanWatched = p.obs.watching() == 1
	return p.plan, nil
}

func fullSpinePlan() *router.PhasePlan {
	return &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "build", Run: true},
		{Phase: "audit", Run: true}, {Phase: "ship", Run: true},
	}}
}

func TestRouterObserver_InitialPlanRunsUnderARouterObserver(t *testing.T) {
	t.Parallel()
	obs := &routerWatchObserver{}
	pl := &observedPlanner{obs: obs, plan: fullSpinePlan()}
	cfg := shadowCfg(config.StageAdvisory)
	cfg.Mode = config.ModeDynamicLLM
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithRouting(cfg, router.StaticPreset{}), WithPlanner(pl), WithObserver(obs))
	ws := t.TempDir()

	o.planCycle(context.Background(), CycleRequest{ProjectRoot: ws}, State{}, CycleState{WorkspacePath: ws, RunID: "run-12"}, 12)

	if pl.planCalls != 1 || !pl.planWatched {
		t.Fatalf("Plan calls=%d watched=%v; the router dispatch must run inside a started observer", pl.planCalls, pl.planWatched)
	}
	assertRouterWatch(t, obs, ws, 12, "run-12")
}

func TestRouterObserver_PostScoutRePlanRunsUnderARouterObserver(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "handoff-scout.json"),
		[]byte(`{"cycle_size_estimate":"large","item1_x":"a","item2_y":"b"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	obs := &routerWatchObserver{}
	pl := &observedPlanner{obs: obs, plan: fullSpinePlan()}
	o := replanOrchestrator(t, &fakeLedger{}, &replanPlanner{})
	o.planner = pl
	o.observer = obs
	cr := &cycleRun{
		o: o, ctx: context.Background(), req: CycleRequest{ProjectRoot: ws}, cycle: 5,
		cs:          CycleState{WorkspacePath: ws, CompletedPhases: []string{"scout"}, RunID: "run-5"},
		envSnap:     map[string]string{},
		clampedPlan: &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: "scout", Run: true}}},
	}

	cr.postScoutReplan()

	if pl.rePlanCallCount != 1 || !pl.rePlanWatched {
		t.Fatalf("RePlan calls=%d watched=%v; the router re-plan must run inside a started observer", pl.rePlanCallCount, pl.rePlanWatched)
	}
	assertRouterWatch(t, obs, ws, 5, "run-5")
}

func assertRouterWatch(t *testing.T, obs *routerWatchObserver, ws string, cycle int, runID string) {
	t.Helper()
	obs.mu.Lock()
	defer obs.mu.Unlock()
	if len(obs.starts) != 1 || obs.starts[0] != "router" {
		t.Fatalf("observer starts = %v, want exactly [router]", obs.starts)
	}
	if obs.cancels != 1 || obs.active != 0 {
		t.Errorf("router observer cancels=%d active=%d, want it stopped once the router returns", obs.cancels, obs.active)
	}
	req := obs.requests[0]
	if req.Workspace != ws || req.Cycle != cycle || req.RunID != runID {
		t.Errorf("router observer request = %+v, want workspace %s cycle %d run %s", req, ws, cycle, runID)
	}
}
