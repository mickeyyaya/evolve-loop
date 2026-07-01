package bridge

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

// Adversarial amplification tests for the LivenessState → reviewer integration
// (cycle-423). Written by the Test Amplifier — black-box view, spec only.
//
// Coverage gaps targeted (not reached by ACS predicates C423_007–C423_010):
//   B1  State=Idle → never ReviewExtend (ACS only specs Converging and Hung paths)
//   B2  State=BusyButStagnant under maxExtends → ReviewExtend (bounded budget path)
//   B3  State=BusyButStagnant at/past maxExtends → NOT ReviewExtend (backstop, cycles 254/255)
//   B4  State=Hung at attempt=0 → fast-fail (AC10 only tests attempt=1)
//   B5  StopEvent.State zero-value ignores the retired legacy Progressed/Busy
//       fields (S3: the boolean fallback is retired, not backward-compat)
//   B6  Converging with non-positive maxExtends still extends (mirrors pre-existing Progressed test)

// TestAmp_Reviewer_IdleStateNotExtend verifies that State=LivenessIdle never
// produces ReviewExtend at any attempt count. Idle means the pane has no busy
// affordance and no new content — it is inactive. The ACS specifies Converging→extend
// and Hung→not extend; Idle behavior maps to the old "Progressed=false, Busy=false"
// path which always paused immediately.
func TestAmp_Reviewer_IdleStateNotExtend(t *testing.T) {
	r := NewDeterministicReviewer(6) // maxExtends=6
	for _, attempt := range []int{0, 1, 3, 6, 9} {
		ev := StopEvent{State: panestream.LivenessIdle, Attempt: attempt}
		verdict := r.Review(ev)
		if verdict.Action == ReviewExtend {
			t.Errorf("Idle at attempt=%d (maxExtends=6): got ReviewExtend, want non-extend (inactive pane must not be extended)", attempt)
		}
	}
}

// TestAmp_Reviewer_BusyButStagnantUnderMaxExtendsExtends verifies that
// State=BusyButStagnant at attempt < maxExtends produces ReviewExtend.
// This is the bounded "busy extension budget": a pane showing a spinner but
// no new content is still alive and deserves wait time (up to the cap).
// The ACS does not explicitly specify this path; it must match the pre-existing
// Busy=true, Progressed=false behavior documented in TestDeterministicReviewer_BusyPaneIsLiveness.
func TestAmp_Reviewer_BusyButStagnantUnderMaxExtendsExtends(t *testing.T) {
	const maxExtends = 4
	r := NewDeterministicReviewer(maxExtends)
	// attempts strictly under the cap: 0, 1, maxExtends-1
	for _, attempt := range []int{0, 1, maxExtends - 1} {
		ev := StopEvent{State: panestream.LivenessBusyButStagnant, Attempt: attempt}
		verdict := r.Review(ev)
		if verdict.Action != ReviewExtend {
			t.Errorf("BusyButStagnant at attempt=%d (maxExtends=%d): got %q, want ReviewExtend (under budget)", attempt, maxExtends, verdict.Action)
		}
	}
}

// TestAmp_Reviewer_BusyButStagnantAtMaxExtendsBackstops verifies that
// State=BusyButStagnant at attempt >= maxExtends produces a non-extend verdict.
// This is the critical backstop for cycles 254/255: a busy-but-stagnant agent
// must be killed when its extension budget is exhausted. Unlike Converging, which
// extends unconditionally (AC9), BusyButStagnant IS bounded by maxExtends.
func TestAmp_Reviewer_BusyButStagnantAtMaxExtendsBackstops(t *testing.T) {
	const maxExtends = 3
	r := NewDeterministicReviewer(maxExtends)
	// attempts at or past the cap
	for _, attempt := range []int{maxExtends, maxExtends + 1, maxExtends + 5} {
		ev := StopEvent{State: panestream.LivenessBusyButStagnant, Attempt: attempt}
		verdict := r.Review(ev)
		if verdict.Action == ReviewExtend {
			t.Errorf("BusyButStagnant at attempt=%d (maxExtends=%d): got ReviewExtend, want non-extend (backstop must fire at/past cap)", attempt, maxExtends)
		}
	}
}

// TestAmp_Reviewer_HungAtAttemptZeroFastFails verifies that State=LivenessHung
// at attempt=0 produces a non-extend verdict. AC10 tests attempt=1 (well under
// maxExtends=6); attempt=0 is the EARLIEST possible detection and must also
// fast-fail — a hung agent should never receive even a single extension.
func TestAmp_Reviewer_HungAtAttemptZeroFastFails(t *testing.T) {
	r := NewDeterministicReviewer(6) // maxExtends=6
	ev := StopEvent{State: panestream.LivenessHung, Attempt: 0}
	verdict := r.Review(ev)
	if verdict.Action == ReviewExtend {
		t.Errorf("Hung at attempt=0 (maxExtends=6): got ReviewExtend, want non-extend (Hung must fast-fail even at first interval)")
	}
}

// TestAmp_Reviewer_StateZeroIgnoresRetiredLegacyFields (S3) verifies that a
// StopEvent with its State field at the zero value NEVER extends, regardless of
// what the legacy Progressed/Busy booleans carry — the pre-S3 fallback that
// derived LivenessState from those booleans is retired (the driver always
// supplies State via panestream.SignalCenter now; an actually-unset State
// carries no liveness signal at all). Supersedes
// TestAmp_Reviewer_BackwardCompatStateZeroUsesLegacyFields, which pinned the
// opposite (now-retired) behavior.
func TestAmp_Reviewer_StateZeroIgnoresRetiredLegacyFields(t *testing.T) {
	r := NewDeterministicReviewer(2)

	// Progressed=true, State=0: the retired fallback would have extended; must
	// now pause.
	progEv := StopEvent{Progressed: true, Attempt: 0}
	if got := r.Review(progEv).Action; got != ReviewPause {
		t.Errorf("State=0, Progressed=true, attempt=0: got %q, want ReviewPause (boolean fallback retired)", got)
	}

	// Same past maxExtends — still must pause; Progressed carries no weight now.
	progPastCap := StopEvent{Progressed: true, Attempt: 9}
	if got := r.Review(progPastCap).Action; got != ReviewPause {
		t.Errorf("State=0, Progressed=true, attempt=9: got %q, want ReviewPause (boolean fallback retired)", got)
	}

	// Legacy idle shape: Progressed=false, Busy=false → pause (unchanged outcome,
	// but now via the default/zero-State case, not a boolean read).
	idleEv := StopEvent{Progressed: false, Busy: false, Attempt: 0}
	if got := r.Review(idleEv).Action; got == ReviewExtend {
		t.Errorf("State=0, Progressed=false, Busy=false, attempt=0: got ReviewExtend, want non-extend")
	}
}

// TestAmp_Reviewer_ConvergingWithNonPositiveMaxExtendsStillExtends verifies that
// State=Converging extends regardless of the maxExtends constructor argument,
// including non-positive values. TestDeterministicReviewer_NonPositiveMaxFallsBack
// covers the legacy Progressed=true path; this covers the new Converging path.
func TestAmp_Reviewer_ConvergingWithNonPositiveMaxExtendsStillExtends(t *testing.T) {
	for _, max := range []int{0, -1} {
		r := NewDeterministicReviewer(max)
		ev := StopEvent{State: panestream.LivenessConverging, Attempt: 0}
		if got := r.Review(ev).Action; got != ReviewExtend {
			t.Errorf("NewDeterministicReviewer(%d): Converging at attempt=0 got %q, want ReviewExtend (converging must always extend regardless of maxExtends)", max, got)
		}
	}
}
