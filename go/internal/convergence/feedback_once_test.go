package convergence_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
)

func TestFeedbackChange_OnceOnlyAtMaxFixRoundsFour(t *testing.T) {
	h := func(prefix string) []convergence.Finding {
		return many(prefix, 2, convergence.SeverityHigh, ofClass("correctness"))
	}
	m := func(prefix string, n int) []convergence.Finding { return many(prefix, n, convergence.SeverityMedium) }
	rounds := [][]convergence.Finding{
		many("h0", 3, convergence.SeverityHigh, ofClass("correctness")),
		join(asFixed(many("h0", 3, convergence.SeverityHigh, ofClass("correctness"))), h("h1")),
		join(asFixed(h("h1")), m("m2", 2)),
		join(asFixed(m("m2", 2)), []convergence.Finding{finding("h3", convergence.SeverityHigh, ofClass("correctness"))}),
	}
	wantRaise := map[int]bool{1: true, 2: false, 3: false}
	wantRung := map[int]int{1: 1, 2: 1, 3: 2}

	for round := 1; round <= 3; round++ {
		in := inputFor(convergence.LoopConsoleLane, rounds[:round+1]...)
		in.Config = withBudget(4)
		in.Fixer = convergence.TierEffort{Family: "claude-tmux", Tier: "balanced", Model: "sonnet", Effort: "medium"}
		in.Judge, in.Headroom = deepJudge, v3bTables

		d := decide(t, in)

		requireOutcome(t, d, convergence.ActionContinue, wantRung[round])
		raised := d.FixerRaise != noRaise || d.JudgeRaise != noRaise
		if raised != wantRaise[round] {
			t.Fatalf("round %d: fixer raise %+v, judge raise %+v, want a raise: %v", round, d.FixerRaise, d.JudgeRaise, wantRaise[round])
		}
		if wantRaise[round] && (d.FixerRaise == noRaise || d.JudgeRaise == noRaise) {
			t.Fatalf("round %d: the single feedback change must raise the fixer and the judge together, got %+v and %+v", round, d.FixerRaise, d.JudgeRaise)
		}
	}
}
