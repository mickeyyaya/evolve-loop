package core

import (
	"encoding/json"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func documentReleaseCfg() config.RoutingConfig {
	return config.RoutingConfig{
		Stage:       config.StageAdvisory,
		Mandatory:   []string{"scout", "build", "audit", "ship"},
		Order:       []string{"scout", "triage", "tdd", "build", "audit", "ship"},
		Conditional: map[string]config.CondRule{"tdd": config.DefaultTddRule()},
	}
}

func persistedFloorForcedPlan(t *testing.T, cfg config.RoutingConfig) *router.PhasePlan {
	t.Helper()
	var advisor []router.PhasePlanEntry
	for _, phase := range cfg.Order {
		advisor = append(advisor, router.PhasePlanEntry{Phase: phase, Run: phase != "tdd"})
	}
	clamped, _ := router.ClampPlanToFloor(router.RouteInput{Cfg: cfg}, &router.PhasePlan{Entries: advisor})
	raw, err := json.Marshal(clamped.Entries)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := parsePhasePlan(string(raw))
	if err != nil {
		t.Fatalf("parsePhasePlan: %v", err)
	}
	return plan
}

func laneSignals(kind string) router.RoutingSignals {
	return router.RoutingSignals{
		Scout:  router.ScoutSignals{Present: true, GoalType: "feature", DeliverableKind: kind, ItemCount: 1},
		Triage: router.TriageSignals{Present: true, CycleSize: "small", DeliverableKind: kind},
	}
}

func TestDocumentLane_TheOrchestratorNeverDispatchesAFloorForcedTDDAfterThePlanRoundTrips(t *testing.T) {
	t.Parallel()
	cfg := documentReleaseCfg()
	o := &Orchestrator{sm: NewStateMachine(), cfg: cfg}
	plan := persistedFloorForcedPlan(t, cfg)

	for kind, want := range map[string]Phase{router.DeliverableKindDocument: PhaseBuild, router.DeliverableKindCode: PhaseTDD} {
		sig := laneSignals(kind)
		dec := router.Route(router.RouteInput{Current: "triage", Verdict: VerdictPASS, Completed: []string{"scout", "triage"}, Signals: sig, Cfg: cfg, Plan: plan}, nil)

		if got, _ := o.enforceNext(PhaseTriage, PhaseTDD, VerdictPASS, sig, dec, true); got != want {
			t.Errorf("%s lane: next = %s, want %s (the persisted plan keeps forced_by, so the released tdd is skipped only for a document)", kind, got, want)
		}
	}
}

func TestSkipsOnResume_AReleasedFloorForcedTDDIsSkipped(t *testing.T) {
	t.Parallel()
	forced := router.PhasePlanEntry{Phase: "tdd", Run: true, ForcedBy: router.RuleShipRequiresTdd}
	planned := router.PhasePlanEntry{Phase: "tdd", Run: true}

	if !skipsOnResume(forced, map[string]bool{}) || skipsOnResume(forced, map[string]bool{"tdd": true}) || skipsOnResume(planned, map[string]bool{}) {
		t.Fatal("a floor-forced tdd is skipped on resume only when the resume floor released it; an advisor-planned tdd never is")
	}
}
