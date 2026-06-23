package ship

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/phases/registry"
)

// TestShipSelfRegisters asserts the ship phase publishes its own factory to the
// phase registry in package init() — exactly like every other built-in phase
// (intent/scout/...). The flow/dispatcher must never know HOW to construct
// ship; it looks the factory up by name. This is the phase-agnostic invariant
// (ADR-0035/0038): adding/owning a phase lives in the phase's package + JSON,
// never in a dispatcher switch.
//
// Registration previously lived in the dispatcher (internal/cli/phasecmd);
// ship now self-registers in its own init(). This is the permanent regression
// guard for that invariant — the test does not import the dispatcher.
func TestShipSelfRegisters(t *testing.T) {
	factory, ok := registry.For(string(core.PhaseShip))
	if !ok {
		t.Fatalf("ship not self-registered: registry.For(%q) returned ok=false", core.PhaseShip)
	}
	runner := factory(core.PhaseRequest{})
	if runner == nil {
		t.Fatal("ship factory returned a nil runner")
	}
	if got := runner.Name(); got != string(core.PhaseShip) {
		t.Errorf("ship factory runner Name() = %q, want %q", got, core.PhaseShip)
	}
}
