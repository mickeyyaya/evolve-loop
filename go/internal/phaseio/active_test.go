package phaseio

import "testing"

func TestPhaseInput_Active(t *testing.T) {
	if (PhaseInput{}).Active() {
		t.Error("zero PhaseInput must be inactive")
	}
	if !NewPhaseInput(PhaseInputInit{Phase: "build"}).Active() {
		t.Error("assembled PhaseInput (Phase set) must be active")
	}
	if NewPhaseInput(PhaseInputInit{}).Active() {
		t.Error("PhaseInput without Phase must be inactive")
	}
}
