package router

// Cycle-238 advisory-soak defect D4: with EVOLVE_REQUIRE_INTENT=1 the static
// state machine starts at intent (NextFromStart), but the advisory plan —
// computed without the requirement — omits intent, so enforceNext's
// plan-honoring override silently drops the operator's required intent gate.
// The fix: the integrity-floor clamp honors RouteInput.IntentRequired and
// forces an intent Run:true entry (clamp rule "require-intent"), mirroring how
// the ship-chain phases are forced. Unlike the ship floor, the intent
// requirement is NOT gated on planRuns(ship): EVOLVE_REQUIRE_INTENT=1 demands
// the intent gate on every cycle shape, exactly as NextFromStart does on the
// static path — a no-ship investigation cycle with the flag set still starts
// at intent.

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

// TestClampPlanToFloor_IntentForcedWhenRequired: a ship plan that omits intent
// gets an intent Run:true entry forced, recorded as a "require-intent" clamp.
func TestClampPlanToFloor_IntentForcedWhenRequired(t *testing.T) {
	out, clamps := ClampPlanToFloor(intentRequiredIn(), fullShipPlan())
	if !planRuns(out, "intent") {
		t.Errorf("intentRequired ship plan must force intent to run; plan=%+v", out.Entries)
	}
	if !clampsHave(clamps, "require-intent") {
		t.Errorf("expected require-intent clamp, got %+v", clamps)
	}
}

// TestClampPlanToFloor_IntentExplicitSkipOverridden: an advisor that
// EXPLICITLY emits intent run:false cannot override the operator's
// EVOLVE_REQUIRE_INTENT — the clamp flips the entry to Run:true.
func TestClampPlanToFloor_IntentExplicitSkipOverridden(t *testing.T) {
	out, clamps := ClampPlanToFloor(intentRequiredIn(), fullShipPlan(pe("intent", false)))
	if !planRuns(out, "intent") {
		t.Errorf("explicit intent run:false must be overridden when required; plan=%+v", out.Entries)
	}
	if !clampsHave(clamps, "require-intent") {
		t.Errorf("expected require-intent clamp, got %+v", clamps)
	}
}

// TestClampPlanToFloor_IntentNotForcedWhenNotRequired: without the operator
// requirement, intent stays whatever the advisor decided (here: omitted) — no
// new always-on phase sneaks into every plan.
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

// TestClampPlanToFloor_IntentAlreadyPlannedNoClamp: an advisor that already
// schedules intent needs no forcing — the clamp completes, never re-records.
func TestClampPlanToFloor_IntentAlreadyPlannedNoClamp(t *testing.T) {
	out, clamps := ClampPlanToFloor(intentRequiredIn(), fullShipPlan(pe("intent", true)))
	if !planRuns(out, "intent") {
		t.Errorf("already-planned intent must keep running; plan=%+v", out.Entries)
	}
	if clampsHave(clamps, "require-intent") {
		t.Errorf("no clamp expected when intent already planned, got %+v", clamps)
	}
}

// TestClampPlanToFloor_IntentForcedOnNoShipPlan: the intent requirement is
// independent of the ship implication — a no-ship plan with
// EVOLVE_REQUIRE_INTENT=1 still gets intent forced, while the ship-floor
// phases stay unforced (the no-ship antecedent is still false for them).
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
