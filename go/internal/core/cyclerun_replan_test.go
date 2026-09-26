package core

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestCycleLoop_PostScoutHookFiresOncePreBuild(t *testing.T) {
	// NOT parallel: mutates the package-level probe seam.
	var fires int
	var completedAtFire [][]string
	prev := postScoutReplanProbe
	postScoutReplanProbe = func(cr *cycleRun) {
		fires++
		completedAtFire = append(completedAtFire, slices.Clone(cr.cs.CompletedPhases))
	}
	t.Cleanup(func() { postScoutReplanProbe = prev })

	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	if fires != 1 {
		t.Fatalf("post-scout hook fired %d times, want exactly 1 (the cycle runs scout once)", fires)
	}
	cp := completedAtFire[0]
	if !slices.Contains(cp, "scout") {
		t.Errorf("hook fired before scout was recorded: completed=%v", cp)
	}
	if slices.Contains(cp, "build") {
		t.Errorf("hook fired post-build: completed=%v (must be pre-build)", cp)
	}
}

type replanPlanner struct {
	replanCalls int
	gotSignals  router.RoutingSignals
	plan        *router.PhasePlan
}

func (p *replanPlanner) Plan(router.RouteInput) (*router.PhasePlan, error) { return p.plan, nil }
func (p *replanPlanner) RePlan(in router.RouteInput) (*router.PhasePlan, error) {
	p.replanCalls++
	p.gotSignals = in.Signals
	return p.plan, nil
}

func replanOrchestrator(t *testing.T, led *fakeLedger, pl *replanPlanner) *Orchestrator {
	t.Helper()
	cfg := shadowCfg(config.StageAdvisory)
	cfg.Mode = config.ModeDynamicLLM
	cfg.RouterReplan = config.StageShadow
	cfg.RePlanMaxDepth = 1
	// A measured-scope trigger: "tester" inserts when scout.item_count >= 1; the
	// initial plans below omit tester, so the measured signals diverge from the
	// plan, producing a material mismatch that triggers a re-plan.
	cfg.Triggers = map[string]config.RoutingBlock{
		"tester": {InsertWhen: []config.Condition{{Field: "scout.item_count", Op: "gte", Value: 1}}},
	}
	return NewOrchestrator(&fakeStorage{}, led, buildRunners(nil), WithRouting(cfg, router.StaticPreset{}), WithPlanner(pl))
}

func TestPlanCycle_RePlanAfterScoutInShadow(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "handoff-scout.json"),
		[]byte(`{"cycle_size_estimate":"large","item1_x":"a","item2_y":"b"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	pl := &replanPlanner{plan: &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "build", Run: true},
		{Phase: "audit", Run: true}, {Phase: "ship", Run: true},
	}}}
	initial := &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: "scout", Run: true}}}
	cr := &cycleRun{
		o: replanOrchestrator(t, &fakeLedger{}, pl), ctx: context.Background(),
		req: CycleRequest{ProjectRoot: ws}, cycle: 5,
		cs:          CycleState{WorkspacePath: ws, CompletedPhases: []string{"scout"}},
		envSnap:     map[string]string{},
		clampedPlan: initial,
	}

	cr.postScoutReplan()

	if pl.replanCalls != 1 {
		t.Fatalf("RePlan called %d times, want 1", pl.replanCalls)
	}
	if !pl.gotSignals.Scout.Present || pl.gotSignals.Scout.ItemCount != 2 {
		t.Errorf("RePlan must get MEASURED scout signals (Present + ItemCount=2); got %+v", pl.gotSignals.Scout)
	}
	if _, err := os.Stat(filepath.Join(ws, "phase-replan.json")); err != nil {
		t.Errorf("shadow re-plan must record phase-replan.json: %v", err)
	}
	if cr.clampedPlan != initial {
		t.Error("shadow re-plan must NOT swap cr.clampedPlan (static still drives)")
	}
}

func TestPlanCycle_NoRePlanWhenSignalsAbsent(t *testing.T) {
	t.Parallel()
	ws := t.TempDir() // no handoff-scout.json ⇒ Scout.Present=false
	pl := &replanPlanner{plan: &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: "scout", Run: true}}}}
	cr := &cycleRun{
		o: replanOrchestrator(t, &fakeLedger{}, pl), ctx: context.Background(),
		req: CycleRequest{ProjectRoot: ws}, cycle: 5,
		cs:      CycleState{WorkspacePath: ws, CompletedPhases: []string{"scout"}},
		envSnap: map[string]string{},
	}

	cr.postScoutReplan()

	if pl.replanCalls != 0 {
		t.Errorf("RePlan must NOT be called when scout signals are absent; calls=%d", pl.replanCalls)
	}
	if _, err := os.Stat(filepath.Join(ws, "phase-replan.json")); err == nil {
		t.Error("no phase-replan.json must be written when the re-plan is skipped")
	}
}

// scoutWorkspace writes a handoff-scout.json with ItemCount=2, which fires the
// mismatch gate (tester inserts at item_count>=1) for a plan that omits tester.
func scoutWorkspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "handoff-scout.json"),
		[]byte(`{"cycle_size_estimate":"large","item1_x":"a","item2_y":"b"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}

func TestRePlan_DepthCappedAndEscalates(t *testing.T) {
	t.Parallel()
	ws := scoutWorkspace(t)
	led := &fakeLedger{}
	pl := &replanPlanner{plan: &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: "scout", Run: true}, {Phase: "build", Run: true}, {Phase: "audit", Run: true}, {Phase: "ship", Run: true}}}}
	cr := &cycleRun{
		o: replanOrchestrator(t, led, pl), ctx: context.Background(),
		req: CycleRequest{ProjectRoot: ws}, cycle: 9,
		cs:          CycleState{WorkspacePath: ws, CompletedPhases: []string{"scout"}},
		envSnap:     map[string]string{},
		clampedPlan: &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: "scout", Run: true}}},
	}

	cr.postScoutReplan() // depth 0 → re-plans, depth → 1
	cr.postScoutReplan() // depth 1 == cap → escalates, no second re-plan

	if pl.replanCalls != 1 {
		t.Fatalf("RePlan called %d times, want 1 (capped at depth 1)", pl.replanCalls)
	}
	if cr.replanDepth != 1 {
		t.Errorf("replanDepth=%d, want 1", cr.replanDepth)
	}
	var escalations int
	for _, e := range led.entries {
		if e.Kind == "replan_escalation" {
			escalations++
		}
	}
	if escalations != 1 {
		t.Errorf("want exactly 1 replan_escalation marker, got %d", escalations)
	}
}

func TestRePlan_FailOpenToStage1Plan(t *testing.T) {
	t.Parallel()
	ws := scoutWorkspace(t)
	pl := &replanPlanner{} // plan==nil ⇒ RePlan returns (nil, nil) ⇒ treated as no usable re-plan
	initial := &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: "scout", Run: true}}}
	cr := &cycleRun{
		o: replanOrchestrator(t, &fakeLedger{}, pl), ctx: context.Background(),
		req: CycleRequest{ProjectRoot: ws}, cycle: 9,
		cs:          CycleState{WorkspacePath: ws, CompletedPhases: []string{"scout"}},
		envSnap:     map[string]string{},
		clampedPlan: initial,
	}

	cr.postScoutReplan()

	if cr.clampedPlan != initial {
		t.Error("a failed/empty re-plan must leave the initial plan in place")
	}
	if _, err := os.Stat(filepath.Join(ws, "phase-replan.json")); err == nil {
		t.Error("a failed/empty re-plan must not write phase-replan.json")
	}
	if cr.replanDepth != 0 {
		t.Errorf("a failed re-plan must not consume a depth slot; replanDepth=%d", cr.replanDepth)
	}
}

func planEntryRuns(p *router.PhasePlan, phase string) bool {
	for _, e := range p.Entries {
		if e.Phase == phase {
			return e.Run
		}
	}
	return false
}

func countOccurrences(xs []string, want string) int {
	n := 0
	for _, x := range xs {
		if x == want {
			n++
		}
	}
	return n
}

func TestRePlan_AdvisoryReplacesPlanAfterClamp(t *testing.T) {
	t.Parallel()
	ws := scoutWorkspace(t)
	pl := &replanPlanner{plan: &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "build", Run: true}, {Phase: "ship", Run: true}, // no audit
	}}}
	o := replanOrchestrator(t, &fakeLedger{}, pl)
	o.cfg.RouterReplan = config.StageAdvisory
	initial := &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: "scout", Run: true}}}
	cr := &cycleRun{
		o: o, ctx: context.Background(), req: CycleRequest{ProjectRoot: ws}, cycle: 3,
		cs:      CycleState{WorkspacePath: ws, CompletedPhases: []string{"scout"}},
		envSnap: map[string]string{}, clampedPlan: initial,
	}

	cr.postScoutReplan()

	if cr.clampedPlan == initial {
		t.Fatal("advisory re-plan must REPLACE the initial drive plan")
	}
	if !planEntryRuns(cr.clampedPlan, "audit") {
		t.Error("the swapped plan must be the CLAMPED re-plan (floor forced audit)")
	}
	if !planEntryRuns(cr.clampedPlan, "ship") {
		t.Error("the swapped plan should still ship")
	}
}

func TestRePlan_NeverWeakensFloorOnReplace(t *testing.T) {
	t.Parallel()
	ws := scoutWorkspace(t)
	pl := &replanPlanner{plan: &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "ship", Run: true}, // ship with neither build nor audit
	}}}
	o := replanOrchestrator(t, &fakeLedger{}, pl)
	o.cfg.RouterReplan = config.StageAdvisory
	initial := &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: "scout", Run: true}}}
	cr := &cycleRun{
		o: o, ctx: context.Background(), req: CycleRequest{ProjectRoot: ws}, cycle: 3,
		cs:      CycleState{WorkspacePath: ws, CompletedPhases: []string{"scout"}},
		envSnap: map[string]string{}, clampedPlan: initial,
	}

	cr.postScoutReplan()

	p := cr.clampedPlan
	if p == initial {
		t.Fatal("advisory re-plan must replace the initial plan")
	}
	if !planEntryRuns(p, "ship") {
		t.Fatal("the re-plan ships, so the swapped plan must ship (else the floor check is vacuous)")
	}
	if !planEntryRuns(p, "build") || !planEntryRuns(p, "audit") {
		t.Errorf("re-plan reached ship without build+audit — floor weakened: %+v", p.Entries)
	}
}

func TestRePlan_MintRegistrationIdempotent(t *testing.T) {
	t.Parallel()
	o := mintOrchestrator(t, fakeMinter{})
	o.registerMintedPhases(mintPlan("phase-a"))            // stage-1: A
	o.registerMintedPhases(mintPlan("phase-a", "phase-b")) // re-plan: re-mint A + new B

	if _, ok := o.runners[Phase("phase-a")]; !ok {
		t.Error("phase-a must be registered (stage-1 mint)")
	}
	if _, ok := o.runners[Phase("phase-b")]; !ok {
		t.Error("phase-b must be registered (re-plan mint)")
	}
	if n := countOccurrences(o.cfg.Order, "phase-a"); n != 1 {
		t.Errorf("phase-a appears %d times in cfg.Order, want exactly 1 (idempotent re-mint)", n)
	}
	if n := countOccurrences(o.cfg.Order, "phase-b"); n != 1 {
		t.Errorf("phase-b appears %d times in cfg.Order, want exactly 1", n)
	}
}
