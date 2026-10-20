package router

import (
	"slices"
	"testing"
)

func advisorPlanWithoutTDD() *PhasePlan {
	var entries []PhasePlanEntry
	for _, phase := range []string{"scout", "triage", "tdd", "build", "audit", "ship"} {
		entries = append(entries, PhasePlanEntry{Phase: phase, Run: phase != "tdd", Justification: "advisor"})
	}
	return &PhasePlan{Entries: entries}
}

func clampedAtCycleStart(t *testing.T) *PhasePlan {
	t.Helper()
	cfg, _ := realRoutingConfig(t)
	plan, clamps := ClampPlanToFloor(RouteInput{Cfg: cfg}, advisorPlanWithoutTDD())
	if !planRuns(plan, "tdd") || !slices.ContainsFunc(clamps, func(c Clamp) bool { return c.Rule == RuleShipRequiresTdd }) {
		t.Fatalf("plan %+v clamps %+v: with empty signals the floor forces tdd at cycle start", plan, clamps)
	}
	return plan
}

func routeAfterTriage(t *testing.T, plan *PhasePlan, sig RoutingSignals) RouterDecision {
	t.Helper()
	cfg, _ := realRoutingConfig(t)
	return Route(RouteInput{Current: "triage", Verdict: "PASS", Completed: []string{"scout", "triage"}, Signals: sig, Cfg: cfg, Plan: plan}, nil)
}

func TestDocumentLane_AFloorForcedTDDLapsesOnceScoutDeclaresADocument(t *testing.T) {
	dec := routeAfterTriage(t, clampedAtCycleStart(t), documentLaneSignals("strategy-options"))

	if dec.NextPhase == "tdd" || !slices.Contains(dec.SkipPhases, "tdd") {
		t.Fatalf("NextPhase %q SkipPhases %v: the advisor planned no tdd, and the document releases the pin that forced it", dec.NextPhase, dec.SkipPhases)
	}
	if !clampForces(dec, RuleTddPinReleased, "tdd=skip") {
		t.Errorf("Clamps = %+v, want %s forcing tdd=skip, so the release is on record", dec.Clamps, RuleTddPinReleased)
	}
}

func TestCodeLane_AFloorForcedTDDStillRuns(t *testing.T) {
	sig := RoutingSignals{
		Scout:  ScoutSignals{Present: true, GoalType: "bugfix", DeliverableKind: DeliverableKindCode, ItemCount: 1},
		Triage: TriageSignals{Present: true, CycleSize: "small", DeliverableKind: DeliverableKindCode},
	}

	if dec := routeAfterTriage(t, clampedAtCycleStart(t), sig); dec.NextPhase != "tdd" {
		t.Fatalf("NextPhase = %q, want tdd: a code lane keeps the plan-time force", dec.NextPhase)
	}
}

func TestClampPlanToFloor_RecordsWhichEntriesTheFloorForced(t *testing.T) {
	plan := clampedAtCycleStart(t)

	for _, e := range plan.Entries {
		want := ""
		if e.Phase == "tdd" {
			want = RuleShipRequiresTdd
		}
		if e.ForcedBy != want {
			t.Errorf("entry %s ForcedBy = %q, want %q: only the entries the floor turned on carry the rule", e.Phase, e.ForcedBy, want)
		}
	}
}
