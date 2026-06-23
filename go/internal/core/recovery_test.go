package core

// recovery_test.go — PA-DDK DDK-6 (ADR-0060): the recovery successor TARGETS are
// config-driven (retrospective in the registry, debugger in the control seam);
// the decision POLICY stays Go. Tests load config via the fixture and assert the
// resolver consults it, with the literal as the fallback.

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/kerneltest"
)

func TestRecoveryTarget_ConfigDriven(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	o := NewOrchestrator(nil, nil, nil, WithCatalog(ref.Catalog))

	// retro PASS → ship comes from the registry recovery map, not the fallback.
	if got := o.recoveryTarget(PhaseRetro, VerdictPASS, PhaseEnd); got != PhaseShip {
		t.Errorf("retro PASS recovery = %q, want ship (from config)", got)
	}
	// debugger RESHIP → ship comes from the control-seam recovery map.
	if got := o.recoveryTarget(PhaseDebugger, "RESHIP", PhaseEnd); got != PhaseShip {
		t.Errorf("debugger RESHIP recovery = %q, want ship (seam config)", got)
	}
	// An unmapped key falls back to the supplied literal.
	if got := o.recoveryTarget(PhaseRetro, "no-such-recovery-key", PhaseTDD); got != PhaseTDD {
		t.Errorf("unmapped recovery key must fall back to the literal; got %q", got)
	}
}

// TestRecoveryTarget_DegradesWithoutCatalog: a bare orchestrator (no catalog)
// returns the literal fallback — byte-identical to pre-DDK-6.
func TestRecoveryTarget_DegradesWithoutCatalog(t *testing.T) {
	t.Parallel()
	o := NewOrchestrator(nil, nil, nil)
	if got := o.recoveryTarget(PhaseRetro, VerdictPASS, PhaseShip); got != PhaseShip {
		t.Errorf("no-catalog recovery must use the literal fallback; got %q", got)
	}
}
