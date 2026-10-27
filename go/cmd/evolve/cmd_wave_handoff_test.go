package main

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func TestWaveCycleText_AHandedOffCycleShowsItsReason(t *testing.T) {
	t.Parallel()
	got := waveCycleText(wave.Cycle{ID: 2035, Verdict: "FAIL", Termination: "handed-off", HandoffStop: "handoff_blocker: M1"})
	if got != "cycle 2035: verdict FAIL, not shipped, handed-off (handoff_blocker: M1)" {
		t.Errorf("text = %q", got)
	}
}
