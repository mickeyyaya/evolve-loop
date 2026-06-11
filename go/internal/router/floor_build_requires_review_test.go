package router

// floor_build_requires_review_test.go — RED contract for the operator policy
// (2026-06-11): "any form of review phase must be selected for the pipeline;
// if the audit/verdict is passed, the code/changes must ship."
//
// Plan-level half of that guarantee, as the CONVERSE implication of the
// existing ship floor:
//
//	run(build) ⇒ run(audit) ∧ run(ship)
//
// A cycle that BUILDS must schedule review and ship — built work may not
// strand unreviewed or unshipped (the cycle-283 class: completed build
// discarded with audit/ship unreached). No-build investigation cycles stay
// unconstrained (the antecedent is false), preserving the documented
// scout-only legitimacy. The runtime half (audit must PASS bound to the built
// tree before ship commits) remains with audit-binding + EGPS — this clamp is
// the plan-level prefilter, defense in depth, never the sole gate.

import "testing"

// TestClampPlanToFloor_BuildForcesAuditAndShip: a plan that runs build but
// schedules neither audit nor ship must have BOTH forced on. RED today: the
// floor only constrains ship-bound plans, so a build-without-review plan
// passes the clamp untouched.
func TestClampPlanToFloor_BuildForcesAuditAndShip(t *testing.T) {
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{
		pe("scout", true), pe("build", true), pe("audit", false), pe("ship", false),
	}}
	out, clamps := ClampPlanToFloor(in, p)
	if !planRuns(out, "audit") {
		t.Error("RED: build ran without audit — a building cycle must select a review phase (operator policy 2026-06-11)")
	}
	if !planRuns(out, "ship") {
		t.Error("RED: build ran without ship scheduled — reviewed work must ship; built work may not strand")
	}
	if !clampsHave(clamps, "build-requires-audit") {
		t.Errorf("missing clamp record build-requires-audit (clamps=%v)", clamps)
	}
	if !clampsHave(clamps, "build-requires-ship") {
		t.Errorf("missing clamp record build-requires-ship (clamps=%v)", clamps)
	}
}

// TestClampPlanToFloor_BuildVetoedAuditIsOverridden: an explicit advisor veto
// of audit (Run:false) in a building plan is overridden, not honored — review
// is non-vetoable wherever build runs.
func TestClampPlanToFloor_BuildVetoedAuditIsOverridden(t *testing.T) {
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{
		pe("build", true), pe("audit", false), pe("ship", true),
	}}
	out, _ := ClampPlanToFloor(in, p)
	if !planRuns(out, "audit") {
		t.Error("advisor veto of audit survived in a building plan — the review floor must be non-vetoable")
	}
}

// TestClampPlanToFloor_NoBuildStaysUnconstrained: the antecedent guard. A
// scout-only investigation plan (no build, no ship) is untouched — the new
// implication must not outlaw legitimate no-build cycles.
func TestClampPlanToFloor_NoBuildStaysUnconstrained(t *testing.T) {
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{
		pe("scout", true), pe("build", false), pe("ship", false),
	}}
	out, clamps := ClampPlanToFloor(in, p)
	if planRuns(out, "audit") || planRuns(out, "ship") {
		t.Error("no-build plan was constrained — investigation cycles must stay legitimate")
	}
	if len(clamps) != 0 {
		t.Errorf("no-build plan recorded clamps: %v", clamps)
	}
}

// TestClampPlanToFloor_BuildForcedShipPullsFullFloor: once build forces ship
// on, the ship floor itself must follow (tdd pinned on non-trivial cycles) —
// the two implications compose rather than bypass each other.
func TestClampPlanToFloor_BuildForcedShipPullsFullFloor(t *testing.T) {
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{
		pe("scout", true), pe("build", true),
	}}
	out, _ := ClampPlanToFloor(in, p)
	if !planRuns(out, "tdd") {
		t.Error("forced ship did not pull the full ship floor (tdd missing on a non-trivial cycle)")
	}
}
