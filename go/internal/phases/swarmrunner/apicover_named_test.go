package swarmrunner

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

func TestDecorator_NamedConcreteType(t *testing.T) {
	var d *Decorator = New(&fakeInner{name: "build"}, &fakeBridge{}, swarm.ModeWriter, Config{})
	if d == nil {
		t.Fatal("New must return a non-nil *Decorator")
	}
	var pr core.PhaseRunner = d
	if got := pr.Name(); got != "build" {
		t.Errorf("Name() = %q, want build (transparent inner name)", got)
	}
}
