package swarmrunner

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

// TestDecorator_NamedConcreteType names the concrete swarmrunner.Decorator type
// (New returns *Decorator but the bare type is never named in a test) and pins
// that New yields a non-nil *Decorator satisfying core.PhaseRunner whose Name is
// transparent — the inner phase name (swarmrunner.go:58-60).
func TestDecorator_NamedConcreteType(t *testing.T) {
	var d *Decorator = New(&fakeInner{name: "build"}, &fakeBridge{}, swarm.ModeWriter, Config{})
	if d == nil {
		t.Fatal("New must return a non-nil *Decorator")
	}
	if got := d.Name(); got != "build" {
		t.Errorf("Name() = %q, want build (transparent inner name)", got)
	}
	var _ core.PhaseRunner = d // the Decorator must satisfy the runner contract it wraps
}
