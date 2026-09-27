package bridge

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

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

func TestAmp_Reviewer_HungAtAttemptZeroFastFails(t *testing.T) {
	r := NewDeterministicReviewer(6) // maxExtends=6
	ev := StopEvent{State: panestream.LivenessHung, Attempt: 0}
	verdict := r.Review(ev)
	if verdict.Action == ReviewExtend {
		t.Errorf("Hung at attempt=0 (maxExtends=6): got ReviewExtend, want non-extend (Hung must fast-fail even at first interval)")
	}
}

func TestAmp_Reviewer_StateZeroIgnoresRetiredLegacyFields(t *testing.T) {
	r := NewDeterministicReviewer(2)

	// The retired fallback would have extended on Progressed=true; must now pause.
	progEv := StopEvent{Progressed: true, Attempt: 0}
	if got := r.Review(progEv).Action; got != ReviewPause {
		t.Errorf("State=0, Progressed=true, attempt=0: got %q, want ReviewPause (boolean fallback retired)", got)
	}

	// Same past maxExtends — still must pause; Progressed carries no weight now.
	progPastCap := StopEvent{Progressed: true, Attempt: 9}
	if got := r.Review(progPastCap).Action; got != ReviewPause {
		t.Errorf("State=0, Progressed=true, attempt=9: got %q, want ReviewPause (boolean fallback retired)", got)
	}

	// Legacy idle shape (Progressed=false, Busy=false) pauses via the default/zero-State case now, not a boolean read.
	idleEv := StopEvent{Progressed: false, Busy: false, Attempt: 0}
	if got := r.Review(idleEv).Action; got == ReviewExtend {
		t.Errorf("State=0, Progressed=false, Busy=false, attempt=0: got ReviewExtend, want non-extend")
	}
}

func TestAmp_Reviewer_ConvergingWithNonPositiveMaxExtendsStillExtends(t *testing.T) {
	for _, max := range []int{0, -1} {
		r := NewDeterministicReviewer(max)
		ev := StopEvent{State: panestream.LivenessConverging, Attempt: 0}
		if got := r.Review(ev).Action; got != ReviewExtend {
			t.Errorf("NewDeterministicReviewer(%d): Converging at attempt=0 got %q, want ReviewExtend (converging must always extend regardless of maxExtends)", max, got)
		}
	}
}
