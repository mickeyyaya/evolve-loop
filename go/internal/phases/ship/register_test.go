package ship

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
)

func TestShipSelfRegisters(t *testing.T) {
	t.Parallel()
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
