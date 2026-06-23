package main

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/phaseorder"
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
	base := cycleRunArgs("abc123", "", false, "")
	if got := join(base); got != "cycle run --goal-hash abc123" {
		t.Errorf("non-simulate args = %q", got)
	}
	sim := cycleRunArgs("abc123", "", true, "")
	if got := join(sim); got != "cycle run --goal-hash abc123 -simulate" {
		t.Errorf("simulate args = %q, want trailing -simulate", got)
	}
}

// TestCycleRunArgs_ThreadsOutputContractAsGoal: the plan's per-cycle output
// contract must reach the launched cycle as --goal so the scout executes the
// PLANNED removal (Context["goal"]), not a free-chosen task. An empty contract
// omits --goal (back-compat: a goal-hash-only cycle keeps the generic goal).
func TestCycleRunArgs_ThreadsOutputContractAsGoal(t *testing.T) {
	contract := "Delete the 5 dead flags; FlagCeiling toward 35. No new env reads."
	if got, want := join(cycleRunArgs("abc123", contract, false, "")), "cycle run --goal-hash abc123 --goal "+contract; got != want {
		t.Errorf("output-contract threading:\n got %q\nwant %q", got, want)
	}
	if g := join(cycleRunArgs("abc123", "", false, "")); g != "cycle run --goal-hash abc123" {
		t.Errorf("empty contract must omit --goal, got %q", g)
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
