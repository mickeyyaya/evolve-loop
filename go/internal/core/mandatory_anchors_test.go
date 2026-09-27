package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func equalPhases(a, b []Phase) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMandatoryAnchorsFor_PureConfigOrderIntersectMandatory(t *testing.T) {
	t.Parallel()
	cfg := config.RoutingConfig{
		Order:     []string{"intent", "scout", "triage", "tdd", "build-planner", "build", "tester", "audit", "ship", "retrospective"},
		Mandatory: []string{"scout", "triage", "build", "audit", "ship"},
	}
	got := mandatoryAnchorsFor(cfg)
	want := []Phase{PhaseScout, PhaseTriage, PhaseBuild, PhaseAudit, PhaseShip}
	if !equalPhases(got, want) {
		t.Errorf("mandatoryAnchorsFor = %v, want %v", got, want)
	}
}

func TestMandatoryAnchorsFor_FollowsConfigOrder(t *testing.T) {
	t.Parallel()
	cfg := config.RoutingConfig{
		Order:     []string{"audit", "scout", "ship", "build"},
		Mandatory: []string{"scout", "build", "audit", "ship"},
	}
	got := mandatoryAnchorsFor(cfg)
	want := []Phase{PhaseAudit, PhaseScout, PhaseShip, PhaseBuild}
	if !equalPhases(got, want) {
		t.Errorf("mandatoryAnchorsFor = %v, want %v (must follow cfg.Order)", got, want)
	}
}

func TestMandatoryAnchorsFor_DegradesWhenOrderUnset(t *testing.T) {
	t.Parallel()
	got := mandatoryAnchorsFor(config.RoutingConfig{Mandatory: []string{"scout", "build", "audit", "ship"}})
	want := []Phase{PhaseScout, PhaseBuild, PhaseAudit, PhaseShip}
	if !equalPhases(got, want) {
		t.Errorf("mandatoryAnchorsFor (no Order) = %v, want canonical %v", got, want)
	}
}

func TestSpineSatisfiedUpTo_NonAnchorIsUnconstrained(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine()
	cfg := config.RoutingConfig{Mandatory: []string{"scout", "build", "audit", "ship"}}
	for _, p := range []Phase{PhaseTDD, PhaseBuildPlanner, PhaseRetro, PhaseDebugger, Phase("user-check")} {
		if !sm.SpineSatisfiedUpTo(p, router.RoutingSignals{}, cfg) {
			t.Errorf("non-anchor %s must be unconstrained by the spine floor (got false with empty signals)", p)
		}
	}
}

func TestSpineSatisfiedUpTo_TriageMandatoryDoesNotWeakenFloor(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine()
	cfg := config.RoutingConfig{
		Order:     []string{"scout", "triage", "build", "audit", "ship", "retrospective"},
		Mandatory: []string{"scout", "triage", "build", "audit", "ship"},
	}
	noAudit := router.RoutingSignals{
		Scout: router.ScoutSignals{Present: true},
		Build: router.BuildSignals{Present: true},
	}
	full := router.RoutingSignals{
		Scout: router.ScoutSignals{Present: true},
		Build: router.BuildSignals{Present: true},
		Audit: router.AuditSignals{Present: true, Verdict: VerdictPASS},
	}
	if sm.SpineSatisfiedUpTo(PhaseShip, noAudit, cfg) {
		t.Error("ship must require a shippable audit even with triage in the mandatory set")
	}
	if !sm.SpineSatisfiedUpTo(PhaseShip, full, cfg) {
		t.Error("ship must be reachable with the full spine present")
	}
}
