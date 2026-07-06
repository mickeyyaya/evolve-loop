package core

import "testing"

// TestLeakRecoverablePhase_CoversAllWorktreePhases is the scout-mandated
// verification anchor (verifiableBy) for
// decouple-leak-recovery-from-worktree-phase-gate: every phase that runs with
// an active cycle worktree (triage, audit, scout, bug-reproduction, tdd,
// build — per the cycle-564 scout finding, cross-referencing the 9 recorded
// tree-diff-leak cycle failures spanning exactly these phases) must be
// eligible for leak recovery, not just the two WorktreePhase (role-gate
// write-permission) phases.
func TestLeakRecoverablePhase_CoversAllWorktreePhases(t *testing.T) {
	recoverable := []Phase{PhaseTriage, PhaseAudit, PhaseScout, Phase("bug-reproduction"), PhaseTDD, PhaseBuild}
	for _, p := range recoverable {
		if !LeakRecoverablePhase(p) {
			t.Errorf("LeakRecoverablePhase(%q) = false, want true", p)
		}
	}
}

// TestLeakRecoverablePhase_ExcludesPhasesWithoutAnActiveWorktree is the
// negative/edge case: phases that never run with an active cycle worktree
// must NOT opt into recovery — a false positive here would make the new
// predicate a blanket always-true no-op rather than a targeted fix.
func TestLeakRecoverablePhase_ExcludesPhasesWithoutAnActiveWorktree(t *testing.T) {
	nonRecoverable := []Phase{PhaseStart, PhaseIntent, PhaseShip, PhaseRetro, PhaseEnd}
	for _, p := range nonRecoverable {
		if LeakRecoverablePhase(p) {
			t.Errorf("LeakRecoverablePhase(%q) = true, want false", p)
		}
	}
}

// TestWorktreePhase_UnchangedByRecoveryDecoupling is the scout-mandated
// verification anchor (verifiableBy) proving the role-gate write-allowance
// predicate is untouched: WorktreePhase must stay EXACTLY PhaseTDD ||
// PhaseBuild. Widening it in place (instead of adding the sibling
// LeakRecoverablePhase predicate) would be a write-permission escalation
// masquerading as a recovery fix — the scout report's explicit reason for
// requiring a SEPARATE predicate.
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
