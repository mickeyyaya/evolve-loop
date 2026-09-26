package main

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestWireOrchestratorDeps_ContractVerifierReachesEveryPhaseRunner(t *testing.T) {
	d := wiredWithRealPhases(t)
	if len(d.Runners) == 0 {
		t.Fatal("orchDeps.Runners must expose the phase-runner map")
	}
	for phase, r := range d.Runners {
		if phase == core.PhaseShip || phase == core.PhaseRetro {
			continue // no BaseRunner-backed verdict engine
		}
		w, ok := r.(interface{ ContractVerifierWired() bool })
		if !ok {
			t.Errorf("phase %s: runner %T exposes no ContractVerifierWired()", phase, r)
			continue
		}
		if !w.ContractVerifierWired() {
			t.Errorf("phase %s: the verdict engine's verifier is not the contract gate's Reviewer (contract_gate defaults to enforce)", phase)
		}
	}
}
