package bridge

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

func TestLivenessMatrix(t *testing.T) {
	t.Parallel()
	const max = 6
	r := NewDeterministicReviewer(max)

	cases := []struct {
		name string
		ev   StopEvent
		want ReviewAction
	}{
		{"converging attempt=0", StopEvent{State: panestream.LivenessConverging, Attempt: 0}, ReviewExtend},
		{"converging attempt=max", StopEvent{State: panestream.LivenessConverging, Attempt: max}, ReviewExtend},
		{"converging attempt=max+3", StopEvent{State: panestream.LivenessConverging, Attempt: max + 3}, ReviewExtend},

		{"busy-stagnant attempt=0", StopEvent{State: panestream.LivenessBusyButStagnant, Attempt: 0}, ReviewExtend},
		{"busy-stagnant attempt=max-1", StopEvent{State: panestream.LivenessBusyButStagnant, Attempt: max - 1}, ReviewExtend},
		{"busy-stagnant attempt=max", StopEvent{State: panestream.LivenessBusyButStagnant, Attempt: max}, ReviewPause},
		{"busy-stagnant attempt=max+1", StopEvent{State: panestream.LivenessBusyButStagnant, Attempt: max + 1}, ReviewPause},

		{"hung attempt=0", StopEvent{State: panestream.LivenessHung, Attempt: 0}, ReviewPause},
		{"hung attempt=1", StopEvent{State: panestream.LivenessHung, Attempt: 1}, ReviewPause},
		{"hung attempt=max-1", StopEvent{State: panestream.LivenessHung, Attempt: max - 1}, ReviewPause},

		{"idle attempt=0", StopEvent{State: panestream.LivenessIdle, Attempt: 0}, ReviewPause},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := r.Review(c.ev).Action
			if got != c.want {
				t.Errorf("Review(%+v).Action = %q, want %q", c.ev, got, c.want)
			}
		})
	}
}

func TestLivenessMatrix_HungFastFailBeforeMaxExtends(t *testing.T) {
	t.Parallel()
	r := NewDeterministicReviewer(6)
	ev := StopEvent{State: panestream.LivenessHung, Attempt: 1}
	if r.Review(ev).Action == ReviewExtend {
		t.Errorf("Hung at attempt=1 (maxExtends=6): got ReviewExtend, want non-extend — Hung must fast-fail before the backstop")
	}
}

func TestLivenessMatrix_ConvergingUnconditionalPastMaxExtends(t *testing.T) {
	t.Parallel()
	r := NewDeterministicReviewer(2)
	ev := StopEvent{State: panestream.LivenessConverging, Attempt: 9}
	if r.Review(ev).Action != ReviewExtend {
		t.Errorf("Converging at attempt=9 (maxExtends=2): got non-extend, want ReviewExtend — Converging must never be capped")
	}
}

func TestLivenessMatrix_BooleanFallbackRetired(t *testing.T) {
	t.Parallel()
	r := NewDeterministicReviewer(3)
	cases := []struct {
		name string
		ev   StopEvent
	}{
		{"progressed, no State → pause", StopEvent{Progressed: true, Attempt: 0}},
		{"progressed past cap, no State → pause", StopEvent{Progressed: true, Attempt: 9}},
		{"busy, no State → pause", StopEvent{Busy: true, Attempt: 0}},
		{"busy+progressed, no State → pause", StopEvent{Progressed: true, Busy: true, Attempt: 0}},
		{"neither, no State → pause", StopEvent{Attempt: 0}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := r.Review(c.ev).Action; got != ReviewPause {
				t.Errorf("Review(%+v).Action = %q, want %q (retired boolean fallback must not extend)", c.ev, got, ReviewPause)
			}
		})
	}
}
