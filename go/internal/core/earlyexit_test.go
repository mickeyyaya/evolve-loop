package core

// earlyexit_test.go — PA-DDK DDK-7 (ADR-0060): the early-exit set is
// config-driven (per-phase early_exit), with the shipPlanned guard staying Go.
// Phases resolved via the kerneltest fixture — no hardcoded names.

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/kerneltest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// TestCanTerminateEarly_ConfigOverridesLiteral proves the config path WINS over
// the literal (not merely agrees with it): the discovery anchor, which the
// literal switch returns true for, is declared early_exit:false in the catalog
// and must then be NON-early-exit. If the config branch in CanTerminateEarly were
// deleted this fails (the literal would still return true). Phase resolved via
// the fixture — no hardcoded name.
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

// TestCanTerminateEarly_DegradesToLiteral: a bare SM (no catalog) uses the
// literal pre-build set — byte-identical to pre-DDK-7.
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
