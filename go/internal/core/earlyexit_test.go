package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/kerneltest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestCanTerminateEarly_ConfigOverridesLiteral(t *testing.T) {
	t.Parallel()
	no := false
	ref := kerneltest.Load(t)
	anchor := ref.FirstAnchor()
	cat := mustCatalog(t, phasespec.PhaseSpec{Name: anchor, EarlyExit: &no})
	sm := NewStateMachine().WithCatalog(specForCatalog(cat))
	if sm.CanTerminateEarly(phaseFromRouter(anchor), false) {
		t.Error("config early_exit:false must OVERRIDE the literal's early-exit-true for the discovery anchor")
	}
}

func TestCanTerminateEarly_ConfigDriven(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	sm := NewStateMachine().WithCatalog(specForCatalog(ref.Catalog))
	firstAnchor := phaseFromRouter(ref.FirstAnchor())

	if !sm.CanTerminateEarly(firstAnchor, false) {
		t.Error("the discovery anchor declares early_exit — a no-ship cycle may terminate there")
	}
	// The shipPlanned guard (a Go invariant) always blocks early-exit.
	if sm.CanTerminateEarly(firstAnchor, true) {
		t.Error("a ship-intended cycle must NEVER early-exit, regardless of config")
	}
	// The ship terminal does not declare early_exit → cannot early-exit.
	if sm.CanTerminateEarly(phaseFromRouter(ref.ShipTerminal()), false) {
		t.Error("the ship terminal must not be early-exit eligible")
	}
}

func TestCanTerminateEarly_DegradesToLiteral(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine()
	if !sm.CanTerminateEarly(PhaseScout, false) {
		t.Error("literal fallback: scout may early-exit")
	}
	if sm.CanTerminateEarly(PhaseBuild, false) {
		t.Error("literal fallback: build may not early-exit")
	}
}
