package pipelinerates

import (
	"math"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

func TestCompare_ADifferenceUnderTheMDEIsInconclusive(t *testing.T) {
	t.Parallel()

	small := Compare(Rate{K: 2, N: 5}, Rate{K: 3, N: 5})
	large := Compare(Rate{K: 10, N: 200}, Rate{K: 150, N: 200})

	if small.Status != qualitypanel.StatusInconclusive || math.Abs(small.Diff-0.2) > 1e-9 || math.Abs(small.MDE-0.8854) > 0.0005 {
		t.Errorf("2/5 vs 3/5 = %+v, want inconclusive with diff 0.2 under the pooled MDE 0.885", small)
	}
	if large.Status != qualitypanel.StatusOK {
		t.Errorf("5%% vs 75%% at n 200 = %+v, want ok", large)
	}
	r := small.Reading("a:wave:139|b:wave:140")
	if r.Index != "PQ3" || r.Status != qualitypanel.StatusInconclusive || r.N != 5 || r.Method != qualitypanel.MethodMDE {
		t.Errorf("reading = %+v", r)
	}
	if (Rate{}).P() != 0 {
		t.Error("an empty rate is 0")
	}
}

func TestMcNemarExact_SevenToOneDiscordantPairs(t *testing.T) {
	t.Parallel()
	if got := McNemarExact(7, 1); math.Abs(got-18.0/256) > 1e-12 {
		t.Errorf("p(7, 1) = %v, want 18/256 = 0.0703125", got)
	}
	if McNemarExact(1, 7) != McNemarExact(7, 1) || McNemarExact(0, 0) != 1 || McNemarExact(4, 4) != 1 {
		t.Error("the test is two-sided and capped at 1")
	}
}

func TestDiscordant_CountsItemsTriedInBothWindows(t *testing.T) {
	t.Parallel()
	cycles := []Cycle{
		{Wave: 1, Item: "a", Shipped: true}, {Wave: 2, Item: "a"},
		{Wave: 1, Item: "b"}, {Wave: 2, Item: "b", Shipped: true},
		{Wave: 1, Item: "c"}, {Wave: 2, Item: "c"},
		{Wave: 1, Item: "d", Shipped: true},
		{Wave: 2, Item: "e", Shipped: true}, {Wave: 2},
	}
	onlyA, onlyB := Discordant(cycles, Window{From: 1, To: 1}, Window{From: 2, To: 2})
	if onlyA != 1 || onlyB != 1 {
		t.Errorf("discordant = %d, %d; want 1 and 1", onlyA, onlyB)
	}
}

func TestParseWindow(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]Window{"wave:140": {Name: "wave:140", From: 140, To: 140}, "waves:121-130": {Name: "waves:121-130", From: 121, To: 130}} {
		if got, err := ParseWindow(in); err != nil || got != want {
			t.Errorf("ParseWindow(%q) = %+v, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "wave:", "wave:1-2", "waves:5", "waves:9-3", "waves:0-3", "waves:a-3", "waves:3-b", "cycle:5"} {
		if _, err := ParseWindow(bad); err == nil {
			t.Errorf("ParseWindow(%q) must fail", bad)
		}
	}
}

func TestMcNemarExact_StaysFiniteForLargeCounts(t *testing.T) {
	t.Parallel()
	if p := McNemarExact(900, 800); math.IsNaN(p) || p <= 0 || p >= 0.05 {
		t.Errorf("p(900, 800) = %v, want a finite small p-value", p)
	}
}

func TestCompare_NoDataOrADegenerateRateIsInconclusive(t *testing.T) {
	t.Parallel()
	for _, c := range []Comparison{Compare(Rate{K: 0, N: 10}, Rate{K: 0, N: 10}), Compare(Rate{}, Rate{K: 1, N: 1}), Compare(Rate{K: 10, N: 10}, Rate{K: 10, N: 10})} {
		if c.Status != qualitypanel.StatusInconclusive {
			t.Errorf("%+v: status %s, want inconclusive", c, c.Status)
		}
	}
}
