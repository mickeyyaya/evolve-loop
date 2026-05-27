package router

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// tddRuleCfg is the default TDD-pin (EVOLVE_CONDITIONAL_MANDATORY): tdd is
// mandatory unless the cycle is trivial.
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

// TestClampPlanToFloor_NoShipIsUnconstrained proves a no-ship plan is left
// untouched — a scout-only investigation cycle is legitimate (ADR-0024 §1).
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

// TestClampPlanToFloor_ShipWithoutAuditRejected is the keystone adversarial case:
// a plan that reaches ship while skipping the whole chain must have build, audit,
// and (non-trivial) tdd forced on, each recorded as a clamp.
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

// TestClampPlanToFloor_ForcesOnlyMissing proves the clamp completes (not
// rewrites): an already-running chain phase is not re-clamped.
func TestClampPlanToFloor_ForcesOnlyMissing(t *testing.T) {
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{pe("scout", true), pe("tdd", true), pe("build", true), pe("audit", false), pe("ship", true)}}
	out, clamps := ClampPlanToFloor(in, p)
	if !planRuns(out, "audit") {
		t.Errorf("audit must be forced on; plan=%+v", out.Entries)
	}
	if len(clamps) != 1 || clamps[0].Rule != "ship-requires-audit" {
		t.Errorf("expected exactly ship-requires-audit, got %+v", clamps)
	}
}

// TestClampPlanToFloor_TrivialExemptsTDD proves the trivial-cycle TDD exemption:
// build+audit are still forced, but tdd is not.
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

// TestClampPlanToFloor_CompleteChainNoClamp: a fully-scheduled chain is untouched.
func TestClampPlanToFloor_CompleteChainNoClamp(t *testing.T) {
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{pe("scout", true), pe("tdd", true), pe("build", true), pe("audit", true), pe("ship", true)}}
	_, clamps := ClampPlanToFloor(in, p)
	if len(clamps) != 0 {
		t.Errorf("complete chain must not clamp, got %+v", clamps)
	}
}

// TestClampPlanToFloor_AbsentPhaseAppended: a chain phase missing from the plan
// entirely (not just run:false) is appended with run=true + a floor justification.
func TestClampPlanToFloor_AbsentPhaseAppended(t *testing.T) {
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{pe("scout", true), pe("tdd", true), pe("build", true), pe("ship", true)}}
	out, clamps := ClampPlanToFloor(in, p)
	if !planRuns(out, "audit") {
		t.Errorf("absent audit must be appended as run=true; plan=%+v", out.Entries)
	}
	if !clampsHave(clamps, "ship-requires-audit") {
		t.Errorf("expected ship-requires-audit clamp, got %+v", clamps)
	}
	found := false
	for _, e := range out.Entries {
		if e.Phase == "audit" && e.Run && e.Justification == "floor: ship requires audit" {
			found = true
		}
	}
	if !found {
		t.Errorf("appended audit entry missing floor justification: %+v", out.Entries)
	}
}

// TestClampPlanToFloor_ShipFalseNotReached: an explicit ship run:false does not
// trip the implication (ship is not reached).
func TestClampPlanToFloor_ShipFalseNotReached(t *testing.T) {
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{pe("scout", true), pe("build", true), pe("ship", false)}}
	_, clamps := ClampPlanToFloor(in, p)
	if len(clamps) != 0 {
		t.Errorf("ship=false does not reach ship; no clamp expected, got %+v", clamps)
	}
}

// TestClampPlanToFloor_ShipAbsentNotReached: ship missing from the plan entirely
// (not merely run:false) is also not reaching ship — the floor imposes nothing.
func TestClampPlanToFloor_ShipAbsentNotReached(t *testing.T) {
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{pe("scout", true), pe("build", true)}}
	_, clamps := ClampPlanToFloor(in, p)
	if len(clamps) != 0 {
		t.Errorf("ship absent → not reached; no clamp expected, got %+v", clamps)
	}
}

// TestClampPlanToFloor_TrivialViaTriagePath: the trivial exemption fires via the
// AUTHORITATIVE triage signal (the common runtime shape), not just scout's estimate.
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

// TestClampPlanToFloor_DoesNotMutateInput: the clamp is pure — the caller's plan
// is never mutated (it returns a fresh copy).
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

// TestClampPlanToFloor_NilPlan: a nil plan degrades to (nil, nil).
func TestClampPlanToFloor_NilPlan(t *testing.T) {
	out, clamps := ClampPlanToFloor(nonTrivialIn(), nil)
	if out != nil || clamps != nil {
		t.Errorf("nil plan → (nil,nil); got %+v %+v", out, clamps)
	}
}

// TestClampPlanToFloor_NoTDDRuleDefaultsPinned: with no configured TDD-pin rule,
// tdd defaults to pinned (the safer, more-mandatory side).
func TestClampPlanToFloor_NoTDDRuleDefaultsPinned(t *testing.T) {
	in := RouteInput{} // empty cfg: no Conditional rule
	p := &PhasePlan{Entries: []PhasePlanEntry{pe("scout", true), pe("ship", true)}}
	out, clamps := ClampPlanToFloor(in, p)
	if !planRuns(out, "tdd") {
		t.Errorf("no TDD-pin rule must default to pinned (tdd forced); plan=%+v", out.Entries)
	}
	if !clampsHave(clamps, "ship-requires-tdd") {
		t.Errorf("expected ship-requires-tdd clamp, got %+v", clamps)
	}
}
