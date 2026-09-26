package router

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func tddRuleCfg() config.RoutingConfig {
	return config.RoutingConfig{
		Conditional: map[string]config.CondRule{
			"tdd": {Field: "cycle_size", Op: "!=", Value: "trivial"},
		},
	}
}

func nonTrivialIn() RouteInput {
	return RouteInput{Cfg: tddRuleCfg(), Signals: RoutingSignals{Scout: ScoutSignals{CycleSizeEstimate: "medium", Present: true}}}
}
func trivialIn() RouteInput {
	return RouteInput{Cfg: tddRuleCfg(), Signals: RoutingSignals{Scout: ScoutSignals{CycleSizeEstimate: "trivial", Present: true}}}
}

func pe(phase string, run bool) PhasePlanEntry { return PhasePlanEntry{Phase: phase, Run: run} }

func clampsHave(clamps []Clamp, rule string) bool {
	for _, c := range clamps {
		if c.Rule == rule {
			return true
		}
	}
	return false
}

func TestClampPlanToFloor_NoShipIsUnconstrained(t *testing.T) {
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{pe("scout", true), pe("build", false), pe("ship", false)}}
	out, clamps := ClampPlanToFloor(in, p)
	if len(clamps) != 0 {
		t.Errorf("no-ship plan must not clamp, got %+v", clamps)
	}
	if !planRuns(out, "scout") || planRuns(out, "build") || planRuns(out, "ship") {
		t.Errorf("no-ship run set changed: %+v", out.Entries)
	}
}

func TestClampPlanToFloor_ShipWithoutAuditRejected(t *testing.T) {
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{pe("scout", true), pe("ship", true)}}
	out, clamps := ClampPlanToFloor(in, p)
	for _, want := range []string{"tdd", "build", "audit"} {
		if !planRuns(out, want) {
			t.Errorf("ship must force %s to run; plan=%+v", want, out.Entries)
		}
		if !clampsHave(clamps, "ship-requires-"+want) {
			t.Errorf("expected ship-requires-%s clamp, got %+v", want, clamps)
		}
	}
}

func TestClampPlanToFloor_ForcesOnlyMissing(t *testing.T) {
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{pe("scout", true), pe("tdd", true), pe("build", true), pe("audit", false), pe("ship", true)}}
	out, clamps := ClampPlanToFloor(in, p)
	if !planRuns(out, "audit") {
		t.Errorf("audit must be forced on; plan=%+v", out.Entries)
	}
	// build-requires-audit fires before the ship floor, so it claims the single clamp.
	if len(clamps) != 1 || clamps[0].Rule != "build-requires-audit" {
		t.Errorf("expected exactly build-requires-audit, got %+v", clamps)
	}
}

func TestClampPlanToFloor_TrivialExemptsTDD(t *testing.T) {
	in := trivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{pe("scout", true), pe("ship", true)}}
	out, clamps := ClampPlanToFloor(in, p)
	if planRuns(out, "tdd") {
		t.Errorf("trivial cycle: tdd must NOT be forced; plan=%+v", out.Entries)
	}
	if !planRuns(out, "build") || !planRuns(out, "audit") {
		t.Errorf("build+audit must still be forced on a trivial ship; plan=%+v", out.Entries)
	}
	if clampsHave(clamps, "ship-requires-tdd") {
		t.Errorf("trivial cycle must not clamp tdd, got %+v", clamps)
	}
	if !clampsHave(clamps, "ship-requires-build") || !clampsHave(clamps, "ship-requires-audit") {
		t.Errorf("expected build+audit clamps, got %+v", clamps)
	}
}

func TestClampPlanToFloor_CompleteChainNoClamp(t *testing.T) {
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{pe("scout", true), pe("tdd", true), pe("build", true), pe("audit", true), pe("ship", true)}}
	_, clamps := ClampPlanToFloor(in, p)
	if len(clamps) != 0 {
		t.Errorf("complete chain must not clamp, got %+v", clamps)
	}
}

func TestClampPlanToFloor_AbsentPhaseAppended(t *testing.T) {
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{pe("scout", true), pe("tdd", true), pe("build", true), pe("ship", true)}}
	out, clamps := ClampPlanToFloor(in, p)
	if !planRuns(out, "audit") {
		t.Errorf("absent audit must be appended as run=true; plan=%+v", out.Entries)
	}
	if !clampsHave(clamps, "build-requires-audit") {
		t.Errorf("expected build-requires-audit clamp (converse implication fires first), got %+v", clamps)
	}
	found := false
	for _, e := range out.Entries {
		if e.Phase == "audit" && e.Run && e.Justification == "floor: integrity floor requires audit" {
			found = true
		}
	}
	if !found {
		t.Errorf("appended audit entry missing floor justification: %+v", out.Entries)
	}
}

func TestClampPlanToFloor_ShipFalseWithBuildOverridden(t *testing.T) {
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{pe("scout", true), pe("build", true), pe("ship", false)}}
	out, clamps := ClampPlanToFloor(in, p)
	if !planRuns(out, "ship") {
		t.Errorf("ship veto must be overridden when build runs; plan=%+v", out.Entries)
	}
	if !clampsHave(clamps, "build-requires-ship") {
		t.Errorf("expected build-requires-ship clamp, got %+v", clamps)
	}
}

func TestClampPlanToFloor_ShipAbsentWithBuildAppended(t *testing.T) {
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{pe("scout", true), pe("build", true)}}
	out, clamps := ClampPlanToFloor(in, p)
	if !planRuns(out, "ship") {
		t.Errorf("absent ship must be appended when build runs; plan=%+v", out.Entries)
	}
	if !clampsHave(clamps, "build-requires-ship") {
		t.Errorf("expected build-requires-ship clamp, got %+v", clamps)
	}
}

func TestClampPlanToFloor_TrivialViaTriagePath(t *testing.T) {
	in := RouteInput{
		Cfg:     tddRuleCfg(),
		Signals: RoutingSignals{Triage: TriageSignals{CycleSize: "trivial", Present: true}},
	}
	p := &PhasePlan{Entries: []PhasePlanEntry{pe("scout", true), pe("ship", true)}}
	out, clamps := ClampPlanToFloor(in, p)
	if planRuns(out, "tdd") {
		t.Errorf("triage-trivial: tdd must NOT be forced; plan=%+v", out.Entries)
	}
	if clampsHave(clamps, "ship-requires-tdd") {
		t.Errorf("triage-trivial must not clamp tdd, got %+v", clamps)
	}
	if !planRuns(out, "build") || !planRuns(out, "audit") {
		t.Errorf("build+audit still forced on a triage-trivial ship; plan=%+v", out.Entries)
	}
}

func TestClampPlanToFloor_DoesNotMutateInput(t *testing.T) {
	in := nonTrivialIn()
	orig := []PhasePlanEntry{pe("scout", true), pe("ship", true)}
	p := &PhasePlan{Entries: orig}
	snapshot := append([]PhasePlanEntry(nil), orig...)
	ClampPlanToFloor(in, p)
	if !reflect.DeepEqual(p.Entries, snapshot) {
		t.Errorf("input plan mutated: got %+v want %+v", p.Entries, snapshot)
	}
}

func TestClampPlanToFloor_NilPlan(t *testing.T) {
	out, clamps := ClampPlanToFloor(nonTrivialIn(), nil)
	if out != nil || clamps != nil {
		t.Errorf("nil plan → (nil,nil); got %+v %+v", out, clamps)
	}
}

func TestClampPlanToFloor_NoTDDRuleDefaultsPinned(t *testing.T) {
	in := RouteInput{}
	p := &PhasePlan{Entries: []PhasePlanEntry{pe("scout", true), pe("ship", true)}}
	out, clamps := ClampPlanToFloor(in, p)
	if !planRuns(out, "tdd") {
		t.Errorf("no TDD-pin rule must default to pinned (tdd forced); plan=%+v", out.Entries)
	}
	if !clampsHave(clamps, "ship-requires-tdd") {
		t.Errorf("expected ship-requires-tdd clamp, got %+v", clamps)
	}
}

// The fixture floor must omit the evaluator so policy takes its append branch; otherwise this false-greens.
func TestEvaluatorFloorPhase_SingleSource(t *testing.T) {
	t.Parallel()
	floor, _ := policy.Policy{ShipFloor: []string{"build"}}.FloorPhases()
	if got := floor[len(floor)-1]; got != EvaluatorFloorPhase {
		t.Fatalf("policy re-appends %q as the evaluator floor phase; router.EvaluatorFloorPhase = %q — the defense-in-depth twins diverged", got, EvaluatorFloorPhase)
	}
}

func TestClampPlanToFloorWith_DropsUnknownPhaseEntry(t *testing.T) {
	const bogus = "gate-wiring-proof"
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{
		pe("scout", true), pe(bogus, true), pe("build", true), pe("ship", true),
	}}

	out, clamps := ClampPlanToFloorWith(in, p, DefaultShipFloor(), false)

	for _, e := range out.Entries {
		if e.Phase == bogus {
			t.Errorf("unknown phase %q survived the clamp: %+v", bogus, out.Entries)
		}
	}
	var drops int
	for _, c := range clamps {
		if c.Rule == DropUnknownPhaseRule && c.Phase == bogus && c.Forced == bogus+"=drop" {
			drops++
		}
	}
	if drops != 1 {
		t.Errorf("want exactly 1 %q clamp for %q, got %d; clamps=%+v",
			DropUnknownPhaseRule, bogus, drops, clamps)
	}
	for _, want := range []string{"scout", "tdd", "build", EvaluatorFloorPhase, "ship"} {
		if !planRuns(out, want) {
			t.Errorf("known phase %q is not running after the drop: %+v", want, out.Entries)
		}
	}
	if len(p.Entries) != 4 || p.Entries[1].Phase != bogus {
		t.Errorf("input plan was mutated: %+v", p.Entries)
	}
}

func TestClampPlanToFloorWith_KeepsCatalogOfferedPhase(t *testing.T) {
	const offered = "bug-reproduction"
	in := nonTrivialIn()
	in.Catalog = []PhaseCard{{Name: offered}}
	p := &PhasePlan{Entries: []PhasePlanEntry{
		pe("scout", true), pe(offered, true), pe("no-such-phase", true),
	}}

	out, clamps := ClampPlanToFloorWith(in, p, DefaultShipFloor(), false)

	if !planRuns(out, offered) {
		t.Errorf("catalog-offered phase %q was dropped: %+v", offered, out.Entries)
	}
	for _, c := range clamps {
		if c.Rule == DropUnknownPhaseRule && c.Phase == offered {
			t.Errorf("drop clamp fired for the catalog-offered phase %q: %+v", offered, c)
		}
	}
	for _, e := range out.Entries {
		if e.Phase == "no-such-phase" {
			t.Errorf("the genuinely unknown phase survived: %+v", out.Entries)
		}
	}
}
