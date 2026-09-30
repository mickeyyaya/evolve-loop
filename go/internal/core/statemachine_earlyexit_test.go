package core

import "testing"

func TestCanTerminateEarly(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine()
	tests := []struct {
		name        string
		from        Phase
		shipPlanned bool
		want        bool
	}{
		{"scout no-ship convergence is legal", PhaseScout, false, true},
		{"triage no-ship convergence is legal", PhaseTriage, false, true},
		{"scout but ship intended is illegal (must satisfy floor)", PhaseScout, true, false},
		{"triage but ship intended is illegal", PhaseTriage, true, false},
		{"build cannot early-exit even no-ship (work must be evaluated)", PhaseBuild, false, false},
		{"audit cannot early-exit", PhaseAudit, false, false},
		{"intent cannot early-exit", PhaseIntent, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sm.CanTerminateEarly(tt.from, tt.shipPlanned); got != tt.want {
				t.Errorf("CanTerminateEarly(%s, shipPlanned=%v) = %v, want %v", tt.from, tt.shipPlanned, got, tt.want)
			}
		})
	}
}

func TestEarlyExitEdgesAreStructurallyLegal(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine()
	if !sm.CanTransition(PhaseScout, PhaseEnd) {
		t.Error("scout→end must be a structurally legal edge for early-exit")
	}
	if !sm.CanTransition(PhaseTriage, PhaseEnd) {
		t.Error("triage→end must be a structurally legal edge for early-exit")
	}
}

func TestEarlyExit_NeverShipsWithoutFloor(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine()
	allPhases := []Phase{
		PhaseStart, PhaseIntent, PhaseScout, PhaseTriage, PhaseTDD,
		PhaseBuildPlanner, PhaseBuild, PhaseAudit, PhaseShip, PhaseRetro,
		PhaseDebugger, PhaseEnd,
	}
	for _, from := range allPhases {
		if sm.CanTerminateEarly(from, true) {
			t.Errorf("CanTerminateEarly(%s, shipPlanned=true) returned true — a ship-intended cycle must never early-exit", from)
		}
	}
}
