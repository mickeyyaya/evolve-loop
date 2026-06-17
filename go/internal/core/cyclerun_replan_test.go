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

// TestCycleLoop_PostScoutHookFiresOncePreBuild pins the WS2-S0 hook call site
// (ADR-0052): the post-scout re-plan hook fires EXACTLY ONCE per cycle, after
// scout's handoff has been recorded (scout ∈ CompletedPhases) and BEFORE build
// (build ∉ CompletedPhases) — the pre-build ordering that lets the re-plan run
// without contradicting a completed anchor. Uses the postScoutReplanProbe DI seam.
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
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"}); err != nil {
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

// replanPlanner is a fake re-invokable planner: implements router.Planner (Plan)
// and the optional rePlanner (RePlan), recording the signals RePlan received.
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
	// A measured-scope trigger so the WS2-S5 mismatch gate fires: "tester" inserts
	// when scout.item_count >= 1; the initial plans below omit tester, so the
	// measured signals diverge from the plan → a material mismatch → re-plan.
	cfg.Triggers = map[string]config.RoutingBlock{
		"tester": {InsertWhen: []config.Condition{{Field: "scout.item_count", Op: "gte", Value: 1}}},
	}
	return NewOrchestrator(&fakeStorage{}, led, buildRunners(nil), WithRouting(cfg, router.StaticPreset{}), WithPlanner(pl))
}

// TestPlanCycle_RePlanAfterScoutInShadow pins WS2-S3 SHADOW: the post-scout
// re-plan is called with MEASURED scout signals and recorded (phase-replan.json),
// but the cycle's drive plan (cr.clampedPlan) is NOT swapped — static still wins.
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

// TestPlanCycle_NoRePlanWhenSignalsAbsent pins the fail-safe: when scout's
// handoff signals are absent (no measured need), the re-plan is not called and
// nothing is recorded.
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

// scoutWorkspace writes a handoff-scout.json with ItemCount=2 so the WS2-S5
// mismatch gate (tester inserts at item_count>=1) fires for a plan that omits
// tester.
func scoutWorkspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "handoff-scout.json"),
		[]byte(`{"cycle_size_estimate":"large","item1_x":"a","item2_y":"b"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}

// TestRePlan_DepthCappedAndEscalates pins WS2-S5: the re-plan fires while
// cr.replanDepth < cap(=1); at the cap it records a debugger-escalation marker
// and does NOT re-plan again (no thrash on a persistent mismatch).
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

// TestRePlan_FailOpenToStage1Plan pins the fail-safe: a RePlan error leaves the
// initial plan in place (no swap — shadow never swaps anyway), writes no re-plan
// artifact, and does not consume a depth slot.
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
