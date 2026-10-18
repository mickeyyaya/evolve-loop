package convergence_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
)

func disputedRound(t *testing.T, loop convergence.Loop, round []convergence.Finding) convergence.Decision {
	return decide(t, inputFor(loop,
		many("a", 2, convergence.SeverityCritical),
		join(asFixed(many("a", 2, convergence.SeverityCritical)), round)))
}

func TestDisputedReason_NamesTheAdjudicatorOfTheLoop(t *testing.T) {
	for _, tc := range []struct {
		name  string
		loop  convergence.Loop
		round []convergence.Finding
		want  []string
	}{
		{"console lane critical goes to the operator", convergence.LoopConsoleLane,
			[]convergence.Finding{finding("x", convergence.SeverityCritical, disputed)},
			[]string{"loop=console-lane", "count=1", "critical=1", "adjudicator=operator"}},
		{"code-review critical goes to the audit", convergence.LoopCodeReview,
			[]convergence.Finding{finding("x", convergence.SeverityCritical, disputed)},
			[]string{"loop=code-review", "count=1", "critical=1", "adjudicator=audit"}},
		{"count covers every disputed row, critical only the critical ones", convergence.LoopConsoleLane,
			[]convergence.Finding{finding("x", convergence.SeverityCritical, disputed), finding("y", convergence.SeverityHigh, disputed),
				finding("z", convergence.SeverityMedium, disputed)},
			[]string{"count=3", "critical=1", "adjudicator=operator"}},
		{"a disputed HIGH alone is still reported with critical=0", convergence.LoopConsoleLane,
			[]convergence.Finding{finding("y", convergence.SeverityHigh, disputed)},
			[]string{"count=1", "critical=0", "adjudicator=operator"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireReason(t, disputedRound(t, tc.loop, tc.round), "CONVERGENCE_DISPUTED", tc.want...)
		})
	}
}

func TestDisputedReason_LeavesTheDecisionItself(t *testing.T) {
	d := disputedRound(t, convergence.LoopConsoleLane, []convergence.Finding{finding("x", convergence.SeverityCritical, disputed)})

	requireOutcome(t, d, convergence.ActionLand, 0)
	if len(d.File) != 0 || len(d.Defer) != 0 {
		t.Fatalf("file %q defer %q, want the disputed row still left to the adjudicator", d.File, d.Defer)
	}
}

func TestDisputedReason_AbsentWithoutADisputedRowInTheDecidedRound(t *testing.T) {
	for _, round := range map[string][]convergence.Finding{
		"no finding":         nil,
		"open critical only": {finding("x", convergence.SeverityCritical)},
		"fixed row only":     {finding("x", convergence.SeverityHigh, fixed)},
		"low and info rows":  {finding("l", convergence.SeverityLow), finding("i", convergence.SeverityInfo)},
	} {
		for _, loop := range []convergence.Loop{convergence.LoopConsoleLane, convergence.LoopCodeReview} {
			refuseReason(t, disputedRound(t, loop, round), "CONVERGENCE_DISPUTED")
		}
	}
}

func TestDisputedReason_ReadsOnlyTheDecidedRound(t *testing.T) {
	in := inputFor(convergence.LoopConsoleLane,
		[]convergence.Finding{finding("x", convergence.SeverityCritical, disputed)},
		[]convergence.Finding{finding("x", convergence.SeverityCritical, fixed)})

	refuseReason(t, decide(t, in), "CONVERGENCE_DISPUTED")
}

func TestDisputedReason_SitsAfterTheRungAndBeforeTheSettleReasons(t *testing.T) {
	d := decide(t, inputFor(convergence.LoopConsoleLane,
		many("a", 2, convergence.SeverityCritical),
		join(asFixed(many("a", 2, convergence.SeverityCritical)),
			[]convergence.Finding{finding("x", convergence.SeverityCritical, disputed), finding("i", convergence.SeverityInfo)})))

	if len(d.Reasons) < 3 {
		t.Fatalf("reasons %q, want rung, disputed and filed", d.Reasons)
	}
	if reasonFor(d, "CONVERGENCE_RUNG") != d.Reasons[0] {
		t.Fatalf("first reason %q, want the rung", d.Reasons[0])
	}
	disputedAt, filedAt := -1, -1
	for i, r := range d.Reasons {
		switch {
		case r == reasonFor(d, "CONVERGENCE_DISPUTED"):
			disputedAt = i
		case r == reasonFor(d, "CONVERGENCE_FILED"):
			filedAt = i
		}
	}
	if disputedAt < 1 || filedAt <= disputedAt {
		t.Fatalf("disputed at %d, filed at %d in %q, want rung < disputed < filed", disputedAt, filedAt, d.Reasons)
	}
}
