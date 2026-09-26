package router

import "testing"

func intentRequiredIn() RouteInput {
	in := nonTrivialIn()
	in.IntentRequired = true
	return in
}

func fullShipPlan(extra ...PhasePlanEntry) *PhasePlan {
	entries := []PhasePlanEntry{
		pe("scout", true), pe("tdd", true), pe("build", true),
		pe("audit", true), pe("ship", true),
	}
	return &PhasePlan{Entries: append(entries, extra...)}
}

func TestClampPlanToFloor_IntentForcedWhenRequired(t *testing.T) {
	out, clamps := ClampPlanToFloor(intentRequiredIn(), fullShipPlan())
	if !planRuns(out, "intent") {
		t.Errorf("intentRequired ship plan must force intent to run; plan=%+v", out.Entries)
	}
	if !clampsHave(clamps, "require-intent") {
		t.Errorf("expected require-intent clamp, got %+v", clamps)
	}
}

func TestClampPlanToFloor_IntentExplicitSkipOverridden(t *testing.T) {
	out, clamps := ClampPlanToFloor(intentRequiredIn(), fullShipPlan(pe("intent", false)))
	if !planRuns(out, "intent") {
		t.Errorf("explicit intent run:false must be overridden when required; plan=%+v", out.Entries)
	}
	if !clampsHave(clamps, "require-intent") {
		t.Errorf("expected require-intent clamp, got %+v", clamps)
	}
}

func TestClampPlanToFloor_IntentNotForcedWhenNotRequired(t *testing.T) {
	in := nonTrivialIn() // IntentRequired zero-value false
	out, clamps := ClampPlanToFloor(in, fullShipPlan())
	if planRuns(out, "intent") {
		t.Errorf("intent forced without IntentRequired; plan=%+v", out.Entries)
	}
	if clampsHave(clamps, "require-intent") {
		t.Errorf("unexpected require-intent clamp, got %+v", clamps)
	}
}

func TestClampPlanToFloor_IntentAlreadyPlannedNoClamp(t *testing.T) {
	out, clamps := ClampPlanToFloor(intentRequiredIn(), fullShipPlan(pe("intent", true)))
	if !planRuns(out, "intent") {
		t.Errorf("already-planned intent must keep running; plan=%+v", out.Entries)
	}
	if clampsHave(clamps, "require-intent") {
		t.Errorf("no clamp expected when intent already planned, got %+v", clamps)
	}
}

func TestClampPlanToFloor_IntentForcedOnNoShipPlan(t *testing.T) {
	p := &PhasePlan{Entries: []PhasePlanEntry{pe("scout", true)}}
	out, clamps := ClampPlanToFloor(intentRequiredIn(), p)
	if !planRuns(out, "intent") {
		t.Errorf("no-ship plan with IntentRequired must still force intent; plan=%+v", out.Entries)
	}
	if !clampsHave(clamps, "require-intent") {
		t.Errorf("expected require-intent clamp, got %+v", clamps)
	}
	if planRuns(out, "build") || planRuns(out, "audit") {
		t.Errorf("no-ship plan must not force the ship chain; plan=%+v", out.Entries)
	}
}
