package convergence

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func cleanFirstRoundF4() Input {
	return Input{Loop: LoopCodeReview, Round: 0, Rounds: []Judgment{{Index: 0}}, Config: policy.Policy{}.ConvergenceConfig()}
}

func TestDecide_ALandingOntoARoundHoldingAnOpenCriticalIsAStopNotALand(t *testing.T) {
	in := cleanFirstRoundF4()
	if got := Decide(in); got.Action != ActionLand {
		t.Fatalf("precondition: a clean first round lands, got %s", got.Action)
	}

	orig := keptRoundHasOpenCritical
	t.Cleanup(func() { keptRoundHasOpenCritical = orig })
	var askedAbout []int
	keptRoundHasOpenCritical = func(c decider, keep int) bool {
		askedAbout = append(askedAbout, keep)
		return true
	}

	got := Decide(in)

	if got.Action != ActionStop || got.Rung != 3 {
		t.Fatalf("decision = %+v, want a rung-3 stop", got)
	}
	if len(askedAbout) != 1 || askedAbout[0] != 0 {
		t.Errorf("the guard must be asked about the round it would keep: %v", askedAbout)
	}
}
