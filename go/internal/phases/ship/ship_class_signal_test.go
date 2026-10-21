package ship

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestTheRoutersShipClassIsTheClassACycleShipRunsAt(t *testing.T) {
	t.Parallel()
	opts := (&Phase{}).shipOptions(core.PhaseRequest{}, "")
	if string(opts.Class) != router.ShipClassCycle {
		t.Fatalf("a cycle ship runs at class %q but the router digests ship.class as %q", opts.Class, router.ShipClassCycle)
	}
}
