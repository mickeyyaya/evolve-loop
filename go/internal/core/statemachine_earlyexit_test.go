package core

import "testing"

// TestCanTerminateEarly covers the early-exit predicate: the advisor may end a
// cycle early ONLY as a no-ship convergence (e.g. scout found nothing), and
// NEVER as a path that reaches end having intended to ship.
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

// TestEarlyExitEdgesAreStructurallyLegal confirms the guarded scout/triage→end
// edges exist in the allow-list (so the orchestrator's CanTransition check
// passes once CanTerminateEarly has authorized the hop).
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

// TestEarlyExit_NeverShipsWithoutFloor is the property guard: across every
// (from, shipPlanned) combination the predicate authorizes, a ship-intended
// cycle is NEVER allowed to terminate early. This is the safety invariant the
// kernel must defend — early-exit can only ever drop a no-ship cycle.
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
