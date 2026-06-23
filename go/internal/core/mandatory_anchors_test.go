package core

// mandatory_anchors_test.go — PA-BIG S6 (ADR-0058): the spine-anchor ORDER is
// derived from config (cfg.Order ∩ cfg.Mandatory ∩ artifact-anchors), not a
// package literal. The artifact map (which phases gate, and on what) stays
// hardcoded. Byte-identity matters here: SpineSatisfiedUpTo is the non-gameable
// ship floor (invariant #2).

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/config"
	"github.com/mickeyyaya/evolveloop/go/internal/router"
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

// TestMandatoryAnchorsFor_PureConfigOrderIntersectMandatory: the anchors are the
// mandatory phases in cfg.Order's order — derived ENTIRELY from config, no Go
// literal. A never-skip phase like triage IS included (it's mandatory); it acts
// as a no-op anchor because anchorArtifactPresent has no check for it.
func TestMandatoryAnchorsFor_PureConfigOrderIntersectMandatory(t *testing.T) {
	t.Parallel()
	cfg := config.RoutingConfig{
		Order:     []string{"intent", "scout", "triage", "tdd", "build-planner", "build", "tester", "audit", "ship", "retrospective"},
		Mandatory: []string{"scout", "triage", "build", "audit", "ship"},
	}
	got := mandatoryAnchorsFor(cfg)
	want := []Phase{PhaseScout, PhaseTriage, PhaseBuild, PhaseAudit, PhaseShip} // exactly cfg.Order ∩ cfg.Mandatory
	if !equalPhases(got, want) {
		t.Errorf("mandatoryAnchorsFor = %v, want %v", got, want)
	}
}

// TestMandatoryAnchorsFor_FollowsConfigOrder proves the ORDER is config-driven:
// a scrambled cfg.Order yields anchors in that order (not the canonical one).
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

// TestMandatoryAnchorsFor_DegradesWhenOrderUnset: a config with no Order (bare
// SMs, the existing spine tests) degrades to the canonical anchor order —
// byte-identical to the pre-S6 literal.
func TestMandatoryAnchorsFor_DegradesWhenOrderUnset(t *testing.T) {
	t.Parallel()
	got := mandatoryAnchorsFor(config.RoutingConfig{Mandatory: []string{"scout", "build", "audit", "ship"}})
	want := []Phase{PhaseScout, PhaseBuild, PhaseAudit, PhaseShip}
	if !equalPhases(got, want) {
		t.Errorf("mandatoryAnchorsFor (no Order) = %v, want canonical %v", got, want)
	}
}

// TestSpineSatisfiedUpTo_NonAnchorIsUnconstrained pins the deliberate contract
// that replaced precedingAnchorBound: a non-anchor target (user/optional
// insertion, or a post-spine phase) is NOT gated by the spine floor — the
// router's plan + legality gate guard those, and CanTerminateEarly gates end.
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

// TestSpineSatisfiedUpTo_TriageMandatoryDoesNotWeakenFloor: with a production-like
// cfg (triage in the mandatory set + a full Order), triage is admitted as a
// no-op anchor (no artifact gate), so the SHIP floor must STILL require a
// shippable audit — adding a never-skip phase to the mandatory set cannot weaken
// the gate. A naive count-based bound would have shifted the indices and dropped
// audit; the anchor-index walk does not.
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
