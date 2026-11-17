package loopwave

import "testing"

func TestPreemptAmplified_TheHighestOutsideItemWins(t *testing.T) {
	sc := plainBoundaryScenario()
	sc.items = append(sc.items, preemptItem{id: "fresh-mid", weight: 0.9})

	out := runPreempt(t, sc)

	requirePlan(t, out, "keep-high", "fresh-top")
	requireSwap(t, out, "slot-low", "fresh-top")
}

func TestPreemptAmplified_OnlyOneSwapPerPlanReuse(t *testing.T) {
	sc := preemptScenario{
		plan: []string{"slot-low", "slot-mid", "keep-high"},
		items: []preemptItem{
			{id: "slot-low", weight: 0.2},
			{id: "slot-mid", weight: 0.3},
			{id: "keep-high", weight: 0.8},
			{id: "fresh-top", weight: 0.95},
			{id: "fresh-next", weight: 0.94},
		},
		count: 3,
	}

	out := runPreempt(t, sc)

	if len(out.lines) != 1 {
		t.Fatalf("ledger preempt lines = %q, want exactly one", out.lines)
	}
	if out.chain != nil {
		t.Errorf("hash chain broken: %v", out.chain)
	}
}

func TestPreemptAmplified_AnOutsideItemSharingAKeptSlotFileIsSkipped(t *testing.T) {
	sc := plainBoundaryScenario()
	sc.items = []preemptItem{
		{id: "slot-low", weight: 0.3},
		{id: "keep-high", weight: 0.8},
		{id: "fresh-top", weight: 0.95, extra: map[string]any{"files": []string{"pkg/keep-high/x.go"}}},
	}

	out := runPreempt(t, sc)

	requirePlan(t, out, "slot-low", "keep-high")
	requireNoPreemption(t, out)
}

func TestPreemptAmplified_ASingleSlotPlanIsSwapped(t *testing.T) {
	sc := preemptScenario{
		plan:  []string{"slot-low"},
		items: []preemptItem{{id: "slot-low", weight: 0.3}, {id: "fresh-top", weight: 0.95}},
		count: 1,
	}

	out := runPreempt(t, sc)

	requirePlan(t, out, "fresh-top")
	requireSwap(t, out, "slot-low", "fresh-top")
}
