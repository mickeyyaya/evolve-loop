package main

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestWireOrchestrator_ShipMainCIWired(t *testing.T) {
	d := wiredWithRealPhases(t)
	w, ok := d.Runners[core.PhaseShip].(interface{ MainCIWired() bool })
	if !ok {
		t.Fatalf("ship runner %T exposes no MainCIWired accessor", d.Runners[core.PhaseShip])
	}
	if !w.MainCIWired() {
		t.Fatal("the production root must set ship.Config.MainCI")
	}
}
