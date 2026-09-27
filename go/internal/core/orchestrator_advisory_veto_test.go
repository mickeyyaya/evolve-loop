package core

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

type fixedPlanner struct{ plan *router.PhasePlan }

func (p *fixedPlanner) Plan(in router.RouteInput) (*router.PhasePlan, error) {
	return p.plan, nil
}

func TestOrchestrator_AdvisoryPlanVetoSurvivesSpineDecline(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	phaseA := &fakeRunner{name: "phase-a"}
	phaseB := &fakeRunner{name: "phase-b"}
	runners[Phase("phase-a")] = phaseA
	runners[Phase("phase-b")] = phaseB

	cat, _ := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{
		{Name: "phase-a", Optional: true, After: "build"},
		{Name: "phase-b", Optional: true, After: "build"},
	})
	if _, ok := cat.Get("phase-b"); !ok {
		t.Fatal("setup: phase-b missing from catalog after Merge")
	}

	cfg := shadowCfg(config.StageAdvisory)
	cfg.Mode = config.ModeDynamicLLM
	cfg.Order = []string{"scout", "triage", "tdd", "build-planner", "build",
		"tester", "phase-a", "phase-b", "audit", "ship"}

	// fakeRunners write no handoff artifacts, so SpineSatisfiedUpTo(audit)
	// stays false all cycle — the exact decline condition that falls back to
	// the static nextInOrder successor.
	plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "tdd", Run: true},
		{Phase: "build", Run: true}, {Phase: "phase-a", Run: true},
		{Phase: "phase-b", Run: false},
		{Phase: "audit", Run: true}, {Phase: "ship", Run: true},
	}}

	o := NewOrchestrator(st, led, runners,
		WithRouting(cfg, router.StaticPreset{}),
		WithCatalog(cat),
		WithPlanner(&fixedPlanner{plan: plan}))

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot:           projectRoot,
		GoalHash:              "g",
		DisableWorkspaceGuard: true,
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	if phaseA.calls != 1 {
		t.Errorf("phase-a runner calls = %d, want 1 (plan-scheduled phase must run)", phaseA.calls)
	}
	if phaseB.calls != 0 {
		t.Errorf("phase-b runner calls = %d, want 0 (plan run:false veto must survive the enforceNext decline fallback)", phaseB.calls)
	}
	if i := indexOfPhase(res.PhasesRun, "phase-b"); i >= 0 {
		t.Errorf("vetoed phase-b present in PhasesRun=%v", res.PhasesRun)
	}
	if i := indexOfPhase(res.PhasesRun, "audit"); i < 0 {
		t.Errorf("audit absent from PhasesRun=%v (cycle must still reach its evaluator)", res.PhasesRun)
	}
}
