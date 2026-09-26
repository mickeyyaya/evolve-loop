package router

import "testing"

func TestClampPlanToFloorWith_AuditOnlyPermitsBuildlessShip(t *testing.T) {
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{
		{Phase: "scout", Run: true},
		{Phase: "audit", Run: true},
		{Phase: "ship", Run: true},
	}}

	out, clamps := ClampPlanToFloorWith(in, p, []string{"audit"}, false)
	if len(clamps) != 0 {
		t.Errorf("audit-only floor on an already-audited ship plan should clamp nothing, got %v", clamps)
	}
	if planRuns(out, "build") {
		t.Error("audit-only floor must NOT force build on")
	}
	if planRuns(out, "tdd") {
		t.Error("audit-only floor must NOT force tdd on")
	}
}

func TestClampPlanToFloorWith_AuditOnlyStillForcesAudit(t *testing.T) {
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{pe("scout", true), pe("ship", true)}}

	out, clamps := ClampPlanToFloorWith(in, p, []string{"audit"}, false)
	if !planRuns(out, "audit") {
		t.Fatal("audit must be forced even under an audit-only floor")
	}
	if !clampsHave(clamps, "ship-requires-audit") {
		t.Errorf("expected ship-requires-audit clamp, got %v", clamps)
	}
}

func TestClampPlanToFloorWith_SelfSealsAudit(t *testing.T) {
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{pe("scout", true), pe("ship", true)}}
	out, _ := ClampPlanToFloorWith(in, p, []string{"build"}, false) // audit deliberately absent
	if !planRuns(out, "audit") {
		t.Fatal("ClampPlanToFloorWith must self-seal audit even when the floor omits it")
	}
}

func TestClampPlanToFloor_DelegatesToDefault(t *testing.T) {
	in := nonTrivialIn()
	p := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "ship", Run: true}}}
	out, clamps := ClampPlanToFloor(in, p)
	for _, phase := range []string{"tdd", "build", "audit"} {
		if !planRuns(out, phase) {
			t.Errorf("default floor must force %q on a non-trivial ship plan", phase)
		}
	}
	if len(clamps) != 3 {
		t.Errorf("expected 3 clamps (tdd,build,audit), got %d: %v", len(clamps), clamps)
	}
}
