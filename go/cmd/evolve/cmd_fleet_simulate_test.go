package main

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseorder"
)

// TestSimulatePhases_CoversCanonicalOrder guards the no-LLM `-simulate` harness:
// every phase the canonical order can route to must have a PASS runner, else a
// full-cycle walk hits "no runner registered for phase <X>" (the build-planner
// regression that broke `evolve fleet --simulate`).
func TestSimulatePhases_CoversCanonicalOrder(t *testing.T) {
	have := map[core.Phase]bool{}
	for _, p := range simulatePhases() {
		have[p] = true
	}
	for _, name := range phaseorder.HardcodedOrder {
		if !have[core.Phase(name)] {
			t.Errorf("simulatePhases() missing canonical phase %q — a full -simulate walk would fail 'no runner registered'", name)
		}
	}
}

// TestCycleRunArgs_Simulate threads -simulate through to each fleet cycle.
func TestCycleRunArgs_Simulate(t *testing.T) {
	base := cycleRunArgs("abc123", false)
	if got := join(base); got != "cycle run --goal-hash abc123" {
		t.Errorf("non-simulate args = %q", got)
	}
	sim := cycleRunArgs("abc123", true)
	if got := join(sim); got != "cycle run --goal-hash abc123 -simulate" {
		t.Errorf("simulate args = %q, want trailing -simulate", got)
	}
}

func join(a []string) string {
	out := ""
	for i, s := range a {
		if i > 0 {
			out += " "
		}
		out += s
	}
	return out
}
