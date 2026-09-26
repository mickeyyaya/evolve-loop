package main

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Research F22 (cycle 1685): the verdict engine and the deliverables gate
// must be ONE verifier, so the bytes a phase's classification judges are the
// bytes the gate approves (a sole recoverable bad_verdict salvaged, persisted
// and reported before classification). The composition root builds the
// gate's Reviewer after the runners and hands every BaseRunner an accessor
// to it — this is the wiring proof, in the SignalCenterReachesEveryPhaseRunner
// shape.
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
