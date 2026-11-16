package main

import "testing"

import "github.com/mickeyyaya/evolve-loop/go/internal/core"

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

func TestMainCIFetcher_ToleratesAMissingGoMod(t *testing.T) {
	t.Parallel()
	if mainCIFetcher(t.TempDir()) == nil {
		t.Fatal("a root without go.mod must still yield a fetcher")
	}
}

func TestWireOrchestrator_ShipStepRunWired(t *testing.T) {
	d := wiredWithRealPhases(t)
	w, ok := d.Runners[core.PhaseShip].(interface{ StepRunWired() bool })
	if !ok {
		t.Fatalf("ship runner %T exposes no StepRunWired accessor", d.Runners[core.PhaseShip])
	}
	if !w.StepRunWired() {
		t.Fatal("the production root must set ship.Config.StepRun")
	}
}
