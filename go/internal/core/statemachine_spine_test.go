package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestCanTransition_WidenedSkipEdges(t *testing.T) {
	sm := NewStateMachine()
	// Trivial-cycle skip paths are now legal.
	if !sm.CanTransition(PhaseScout, PhaseBuild) {
		t.Errorf("scout→build should be legal (trivial-cycle tdd skip)")
	}
	if !sm.CanTransition(PhaseTriage, PhaseBuild) {
		t.Errorf("triage→build should be legal")
	}
	// Existing invariant preserved: build→ship is NOT a legal direct edge.
	if sm.CanTransition(PhaseBuild, PhaseShip) {
		t.Errorf("build→ship must remain illegal (audit is mandatory between)")
	}
}

func fullSpineCfg() config.RoutingConfig {
	return config.RoutingConfig{Mandatory: []string{"scout", "build", "audit", "ship"}}
}

func TestSpineSatisfiedUpTo_ShipRequiresShippableAudit(t *testing.T) {
	sm := NewStateMachine()
	cfg := fullSpineCfg()

	present := func(auditVerdict string, auditPresent bool) router.RoutingSignals {
		return router.RoutingSignals{
			Scout: router.ScoutSignals{Present: true},
			Build: router.BuildSignals{Present: true},
			Audit: router.AuditSignals{Present: auditPresent, Verdict: auditVerdict},
		}
	}

	if sm.SpineSatisfiedUpTo(PhaseShip, present("WARN", true), cfg) != true {
		t.Errorf("ship should be reachable with a present WARN audit")
	}
	if sm.SpineSatisfiedUpTo(PhaseShip, present("PASS", true), cfg) != true {
		t.Errorf("ship should be reachable with a present PASS audit")
	}
	if sm.SpineSatisfiedUpTo(PhaseShip, present("FAIL", true), cfg) != false {
		t.Errorf("ship must be BLOCKED with a FAIL audit verdict")
	}
	if sm.SpineSatisfiedUpTo(PhaseShip, present("PASS", false), cfg) != false {
		t.Errorf("ship must be BLOCKED when the audit artifact is absent (anti-fabrication)")
	}
}

func TestSpineSatisfiedUpTo_BuildRequiresScout(t *testing.T) {
	sm := NewStateMachine()
	cfg := fullSpineCfg()

	withScout := router.RoutingSignals{Scout: router.ScoutSignals{Present: true}}
	noScout := router.RoutingSignals{}

	if !sm.SpineSatisfiedUpTo(PhaseBuild, withScout, cfg) {
		t.Errorf("build should be reachable when scout artifact present")
	}
	if sm.SpineSatisfiedUpTo(PhaseBuild, noScout, cfg) {
		t.Errorf("build must be BLOCKED when scout artifact absent")
	}
}

func TestSpineSatisfiedUpTo_ConfigurableMandatoryWeakensGate(t *testing.T) {
	sm := NewStateMachine()
	// Operator dropped audit from the mandatory set (weak-spine).
	weak := config.RoutingConfig{Mandatory: []string{"scout", "build", "ship"}}
	sig := router.RoutingSignals{
		Scout: router.ScoutSignals{Present: true},
		Build: router.BuildSignals{Present: true},
		// audit absent
	}
	if !sm.SpineSatisfiedUpTo(PhaseShip, sig, weak) {
		t.Errorf("with audit removed from mandatory set, ship gate should not require audit artifact")
	}
}
