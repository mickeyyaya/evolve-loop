package core

import "testing"

func TestLeakRecoverablePhase_CoversAllWorktreePhases(t *testing.T) {
	recoverable := []Phase{PhaseTriage, PhaseAudit, PhaseScout, Phase("bug-reproduction"), PhaseTDD, PhaseBuild}
	for _, p := range recoverable {
		if !LeakRecoverablePhase(p) {
			t.Errorf("LeakRecoverablePhase(%q) = false, want true", p)
		}
	}
}

func TestLeakRecoverablePhase_ExcludesPhasesWithoutAnActiveWorktree(t *testing.T) {
	nonRecoverable := []Phase{PhaseStart, PhaseIntent, PhaseShip, PhaseRetro, PhaseEnd}
	for _, p := range nonRecoverable {
		if LeakRecoverablePhase(p) {
			t.Errorf("LeakRecoverablePhase(%q) = true, want false", p)
		}
	}
}

func TestWorktreePhase_UnchangedByRecoveryDecoupling(t *testing.T) {
	mustBeTrue := []Phase{PhaseTDD, PhaseBuild}
	for _, p := range mustBeTrue {
		if !WorktreePhase(p) {
			t.Errorf("WorktreePhase(%q) = false, want true", p)
		}
	}
	mustBeFalse := []Phase{PhaseTriage, PhaseAudit, PhaseScout, Phase("bug-reproduction"),
		PhaseShip, PhaseRetro, PhaseEnd, PhaseStart, PhaseIntent}
	for _, p := range mustBeFalse {
		if WorktreePhase(p) {
			t.Errorf("WorktreePhase(%q) = true, want false (role-gate write-allowance must stay tdd/build only)", p)
		}
	}
}
