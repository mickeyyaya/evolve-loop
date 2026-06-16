package phaseio

import "testing"

// TestPhaseInput_Active: Active is the populated-marker the phase readers gate on
// (ADR-0050 §3.10 Slice 2) — true only for an assembled envelope (Phase stamped by
// the dispatch seam), false for the zero value.
func TestPhaseInput_Active(t *testing.T) {
	if (PhaseInput{}).Active() {
		t.Error("zero PhaseInput must be inactive")
	}
	if !NewPhaseInput(PhaseInputInit{Phase: "build"}).Active() {
		t.Error("assembled PhaseInput (Phase set) must be active")
	}
	// The dispatch seam always stamps Phase; an envelope without it is treated as
	// inactive (guards against a hand-built misuse falling through to the typed path).
	if NewPhaseInput(PhaseInputInit{}).Active() {
		t.Error("PhaseInput without Phase must be inactive")
	}
}
