package bridge

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

// liveness_matrix_test.go — T2 behavioral tests: the reviewer maps
// LivenessState → ReviewAction with zero CLI-name literals in stopreview.go.
// These tests pin the four invariants that replaced the coarse boolean path.

// TestLivenessMatrix covers the full LivenessState × ReviewAction decision table.
// All four states are exercised across multiple attempt counts to prove the
// reviewer is driven by State, not by hard-coded per-CLI logic.
func TestLivenessMatrix(t *testing.T) {
	const max = 6 // maxExtends for this test suite
	r := NewDeterministicReviewer(max)

	cases := []struct {
		name string
		ev   StopEvent
		want ReviewAction
	}{
		// Converging → ReviewExtend unconditionally (cycles 311/312: no cap on real output)
		{"converging attempt=0", StopEvent{State: panestream.LivenessConverging, Attempt: 0}, ReviewExtend},
		{"converging attempt=max", StopEvent{State: panestream.LivenessConverging, Attempt: max}, ReviewExtend},
		{"converging attempt=max+3", StopEvent{State: panestream.LivenessConverging, Attempt: max + 3}, ReviewExtend},

		// BusyButStagnant → extend under cap, pause at/over cap (cycles 254/255)
		{"busy-stagnant attempt=0", StopEvent{State: panestream.LivenessBusyButStagnant, Attempt: 0}, ReviewExtend},
		{"busy-stagnant attempt=max-1", StopEvent{State: panestream.LivenessBusyButStagnant, Attempt: max - 1}, ReviewExtend},
		{"busy-stagnant attempt=max", StopEvent{State: panestream.LivenessBusyButStagnant, Attempt: max}, ReviewPause},
		{"busy-stagnant attempt=max+1", StopEvent{State: panestream.LivenessBusyButStagnant, Attempt: max + 1}, ReviewPause},

		// Hung → fast-fail BEFORE maxExtends backstop (new: detector fast-path)
		{"hung attempt=0", StopEvent{State: panestream.LivenessHung, Attempt: 0}, ReviewPause},
		{"hung attempt=1", StopEvent{State: panestream.LivenessHung, Attempt: 1}, ReviewPause},
		{"hung attempt=max-1", StopEvent{State: panestream.LivenessHung, Attempt: max - 1}, ReviewPause},

		// Idle → pause immediately (no liveness signal)
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

// TestLivenessMatrix_HungFastFailBeforeMaxExtends pins the fast-fail invariant:
// a Hung sequence returns non-Extend at attempt=1, which is STRICTLY LESS than
// maxExtends=6. This is the latency win: the detector fast-fails BEFORE the
// ~maxExtends×300s = ~30-min backstop would trigger on BusyButStagnant.
func TestLivenessMatrix_HungFastFailBeforeMaxExtends(t *testing.T) {
	r := NewDeterministicReviewer(6)
	ev := StopEvent{State: panestream.LivenessHung, Attempt: 1}
	if r.Review(ev).Action == ReviewExtend {
		t.Errorf("Hung at attempt=1 (maxExtends=6): got ReviewExtend, want non-extend — Hung must fast-fail before the backstop")
	}
}

// TestLivenessMatrix_ConvergingUnconditionalPastMaxExtends pins the unconditional-
// extend invariant: Converging at attempt=9 (past maxExtends=2) still returns
// ReviewExtend. This closes cycles 311/312: a producing scout was killed at
// the maxExtends backstop even while emitting real output.
func TestLivenessMatrix_ConvergingUnconditionalPastMaxExtends(t *testing.T) {
	r := NewDeterministicReviewer(2)
	ev := StopEvent{State: panestream.LivenessConverging, Attempt: 9}
	if r.Review(ev).Action != ReviewExtend {
		t.Errorf("Converging at attempt=9 (maxExtends=2): got non-extend, want ReviewExtend — Converging must never be capped")
	}
}

// TestLivenessMatrix_BooleanFallbackRetired (S3) verifies the pre-S3
// Progressed+Busy boolean fallback is RETIRED, not merely shadowed: a
// StopEvent with State left at its zero value gets ReviewPause regardless of
// what Progressed/Busy carry — those fields are evidence for fatalpane.go +
// logging now, never a reviewer decision path. Supersedes the old
// TestLivenessMatrix_BackwardCompatBooleans, which pinned the opposite
// (now-retired) behavior.
func TestLivenessMatrix_BooleanFallbackRetired(t *testing.T) {
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
