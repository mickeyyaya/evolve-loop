package integration

import (
	"math"
	"reflect"
	"slices"
	"testing"
	"time"
)

func TestAmplifiedOrderDeliverables_NilAndEmptyInput(t *testing.T) {
	if got := OrderDeliverables(nil); len(got) != 0 {
		t.Fatalf("OrderDeliverables(nil) len = %d, want 0", len(got))
	}

	if got := OrderDeliverables([]Deliverable{}); len(got) != 0 {
		t.Fatalf("OrderDeliverables([]) len = %d, want 0", len(got))
	}

	single := []Deliverable{{ID: "only", InboxRank: 1}}
	if got := OrderDeliverables(single); len(got) != 1 || got[0].ID != "only" {
		t.Fatalf("OrderDeliverables(single) = %+v, want single item", got)
	}

	selfDep := []Deliverable{{ID: "loop", Deps: []string{"loop"}}}
	if got := OrderDeliverables(selfDep); len(got) != 1 || got[0].ID != "loop" {
		t.Fatalf("OrderDeliverables(selfDep) = %+v, want single item", got)
	}

	unknownDep := []Deliverable{{ID: "leaf", Deps: []string{"missing", ""}}}
	if got := OrderDeliverables(unknownDep); len(got) != 1 || got[0].ID != "leaf" {
		t.Fatalf("OrderDeliverables(unknownDep) = %+v, want single item", got)
	}
}

func TestAmplifiedOrderDeliverables_DeepImmutabilityAndPurity(t *testing.T) {
	d1 := Deliverable{ID: "c", Deps: []string{"p"}, InboxRank: 5}
	d2 := Deliverable{ID: "p", InboxRank: 10}
	input := []Deliverable{d1, d2}
	inputCopy := []Deliverable{
		{ID: "c", Deps: []string{"p"}, InboxRank: 5},
		{ID: "p", InboxRank: 10},
	}

	out1 := OrderDeliverables(input)
	if !reflect.DeepEqual(input, inputCopy) {
		t.Fatalf("OrderDeliverables mutated input slice: got %+v, want %+v", input, inputCopy)
	}

	out2 := OrderDeliverables(input)
	if !reflect.DeepEqual(out1, out2) {
		t.Fatalf("OrderDeliverables impure: out1 != out2 across repeated calls on identical input")
	}

	if len(out1) > 0 {
		out1[0].ID = "tampered"
		if len(out1[0].Deps) > 0 {
			out1[0].Deps[0] = "tampered_dep"
		}
	}

	out3 := OrderDeliverables(input)
	if !reflect.DeepEqual(out3, out2) {
		t.Fatalf("Modifying return slice affected subsequent OrderDeliverables output")
	}
}

func TestAmplifiedOrderDeliverables_TopologicalDAGInvariants(t *testing.T) {
	producer := Deliverable{ID: "prod", InboxRank: 10}
	c1 := Deliverable{ID: "c1", Deps: []string{"prod"}, InboxRank: 5}
	c2 := Deliverable{ID: "c2", Deps: []string{"prod"}, InboxRank: 4}
	consumerFinal := Deliverable{ID: "final", Deps: []string{"c1", "c2"}, InboxRank: 1}

	batch := []Deliverable{consumerFinal, c2, c1, producer}
	perms := generateDeliverablePermutations(batch)

	for _, p := range perms {
		ordered := OrderDeliverables(p)
		if len(ordered) != 4 {
			t.Fatalf("OrderDeliverables returned len %d, want 4", len(ordered))
		}
		pos := make(map[string]int, len(ordered))
		for idx, item := range ordered {
			pos[item.ID] = idx
		}
		if pos["prod"] >= pos["c1"] {
			t.Fatalf("producer prod (pos %d) came after c1 (pos %d)", pos["prod"], pos["c1"])
		}
		if pos["prod"] >= pos["c2"] {
			t.Fatalf("producer prod (pos %d) came after c2 (pos %d)", pos["prod"], pos["c2"])
		}
		if pos["c1"] >= pos["final"] {
			t.Fatalf("c1 (pos %d) came after final (pos %d)", pos["c1"], pos["final"])
		}
		if pos["c2"] >= pos["final"] {
			t.Fatalf("c2 (pos %d) came after final (pos %d)", pos["c2"], pos["final"])
		}
	}
}

func TestAmplifiedOrderDeliverables_DeepChainAndDuplicateDeps(t *testing.T) {
	d1 := Deliverable{ID: "d1"}
	d2 := Deliverable{ID: "d2", Deps: []string{"d1", "d1", "d1"}}
	d3 := Deliverable{ID: "d3", Deps: []string{"d2"}}
	d4 := Deliverable{ID: "d4", Deps: []string{"d3"}}
	d5 := Deliverable{ID: "d5", Deps: []string{"d4"}}

	input := []Deliverable{d5, d3, d1, d4, d2}
	ordered := OrderDeliverables(input)

	wantIDs := []string{"d1", "d2", "d3", "d4", "d5"}
	gotIDs := make([]string, len(ordered))
	for i, item := range ordered {
		gotIDs[i] = item.ID
	}

	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("OrderDeliverables chain = %v, want %v", gotIDs, wantIDs)
	}
}

func TestAmplifiedOrderDeliverables_DeterministicCycleResolution(t *testing.T) {
	a := Deliverable{ID: "a", Deps: []string{"b"}, InboxRank: 1}
	b := Deliverable{ID: "b", Deps: []string{"a"}, InboxRank: 2}

	out1 := OrderDeliverables([]Deliverable{a, b})
	out2 := OrderDeliverables([]Deliverable{b, a})

	if len(out1) != 2 || len(out2) != 2 {
		t.Fatalf("Cycle resulted in missing elements: out1 len=%d, out2 len=%d", len(out1), len(out2))
	}

	if !reflect.DeepEqual(out1, out2) {
		t.Fatalf("OrderDeliverables across mutual cycle is not deterministic: out1=%+v, out2=%+v", out1, out2)
	}

	c1 := Deliverable{ID: "c1", Deps: []string{"c3"}, InboxRank: 10}
	c2 := Deliverable{ID: "c2", Deps: []string{"c1"}, InboxRank: 20}
	c3 := Deliverable{ID: "c3", Deps: []string{"c2"}, InboxRank: 30}

	cycle3Perms := generateDeliverablePermutations([]Deliverable{c1, c2, c3})
	var canonical []Deliverable
	for i, p := range cycle3Perms {
		got := OrderDeliverables(p)
		if len(got) != 3 {
			t.Fatalf("3-cycle lost elements: len=%d, want 3", len(got))
		}
		if i == 0 {
			canonical = got
		} else if !reflect.DeepEqual(got, canonical) {
			t.Fatalf("3-cycle permutation %d produced different order %+v vs canonical %+v", i, got, canonical)
		}
	}
}

func TestAmplifiedOrderDeliverables_AllSevenKeysPrecedence(t *testing.T) {
	baseTime := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

	rankLow := Deliverable{ID: "r1", InboxRank: 1, UnblocksCount: 0}
	rankHigh := Deliverable{ID: "r2", InboxRank: 5, UnblocksCount: 100}
	if got := OrderDeliverables([]Deliverable{rankHigh, rankLow}); got[0].ID != "r1" {
		t.Fatalf("Rank tie-break failed: got %s, want r1", got[0].ID)
	}

	unblockHigh := Deliverable{ID: "u1", InboxRank: 1, UnblocksCount: 50, Risky: true}
	unblockLow := Deliverable{ID: "u2", InboxRank: 1, UnblocksCount: 10, Risky: false}
	if got := OrderDeliverables([]Deliverable{unblockLow, unblockHigh}); got[0].ID != "u1" {
		t.Fatalf("UnblocksCount tie-break failed: got %s, want u1", got[0].ID)
	}

	safe := Deliverable{ID: "s", InboxRank: 1, UnblocksCount: 10, Risky: false, VerificationCost: 1000}
	risky := Deliverable{ID: "r", InboxRank: 1, UnblocksCount: 10, Risky: true, VerificationCost: 1}
	if got := OrderDeliverables([]Deliverable{risky, safe}); got[0].ID != "s" {
		t.Fatalf("Risk tie-break failed: got %s, want s", got[0].ID)
	}

	cheap := Deliverable{ID: "cheap", InboxRank: 1, UnblocksCount: 10, Risky: false, VerificationCost: 5, DeliveredAt: baseTime.Add(time.Hour)}
	costly := Deliverable{ID: "costly", InboxRank: 1, UnblocksCount: 10, Risky: false, VerificationCost: 50, DeliveredAt: baseTime}
	if got := OrderDeliverables([]Deliverable{costly, cheap}); got[0].ID != "cheap" {
		t.Fatalf("VerificationCost tie-break failed: got %s, want cheap", got[0].ID)
	}

	early := Deliverable{ID: "early", InboxRank: 1, UnblocksCount: 10, Risky: false, VerificationCost: 5, DeliveredAt: baseTime}
	late := Deliverable{ID: "late", InboxRank: 1, UnblocksCount: 10, Risky: false, VerificationCost: 5, DeliveredAt: baseTime.Add(time.Minute)}
	if got := OrderDeliverables([]Deliverable{late, early}); got[0].ID != "early" {
		t.Fatalf("DeliveredAt tie-break failed: got %s, want early", got[0].ID)
	}

	idAlpha := Deliverable{ID: "alpha", InboxRank: 1, UnblocksCount: 10, Risky: false, VerificationCost: 5, DeliveredAt: baseTime}
	idBeta := Deliverable{ID: "beta", InboxRank: 1, UnblocksCount: 10, Risky: false, VerificationCost: 5, DeliveredAt: baseTime}
	if got := OrderDeliverables([]Deliverable{idBeta, idAlpha}); got[0].ID != "alpha" {
		t.Fatalf("ID ascending tie-break failed: got %s, want alpha", got[0].ID)
	}
}

func TestAmplifiedOrderDeliverables_BoundaryAndExtremeValues(t *testing.T) {
	minRank := Deliverable{ID: "min_rank", InboxRank: math.MinInt}
	maxRank := Deliverable{ID: "max_rank", InboxRank: math.MaxInt}
	if got := OrderDeliverables([]Deliverable{maxRank, minRank}); got[0].ID != "min_rank" {
		t.Fatalf("math.MinInt rank should precede math.MaxInt rank, got %s", got[0].ID)
	}

	maxUnblocks := Deliverable{ID: "max_unblocks", InboxRank: 0, UnblocksCount: math.MaxInt}
	minUnblocks := Deliverable{ID: "min_unblocks", InboxRank: 0, UnblocksCount: math.MinInt}
	if got := OrderDeliverables([]Deliverable{minUnblocks, maxUnblocks}); got[0].ID != "max_unblocks" {
		t.Fatalf("math.MaxInt unblocks should precede math.MinInt unblocks, got %s", got[0].ID)
	}

	minCost := Deliverable{ID: "min_cost", InboxRank: 0, UnblocksCount: 0, VerificationCost: math.MinInt}
	maxCost := Deliverable{ID: "max_cost", InboxRank: 0, UnblocksCount: 0, VerificationCost: math.MaxInt}
	if got := OrderDeliverables([]Deliverable{maxCost, minCost}); got[0].ID != "min_cost" {
		t.Fatalf("math.MinInt cost should precede math.MaxInt cost, got %s", got[0].ID)
	}

	emptyID := Deliverable{ID: "", InboxRank: 0}
	namedID := Deliverable{ID: "a", InboxRank: 0}
	if got := OrderDeliverables([]Deliverable{namedID, emptyID}); got[0].ID != "" {
		t.Fatalf("Empty ID should precede named ID in tie-breaker, got %s", got[0].ID)
	}

	zeroTime := Deliverable{ID: "zero_time", DeliveredAt: time.Time{}}
	epochTime := Deliverable{ID: "epoch_time", DeliveredAt: time.Unix(0, 0)}
	if got := OrderDeliverables([]Deliverable{epochTime, zeroTime}); got[0].ID != "zero_time" {
		t.Fatalf("time.Time{} should precede unix epoch in time comparison, got %s", got[0].ID)
	}
}

func TestAmplifiedOrderDeliverables_PermutationDeterminismStress(t *testing.T) {
	t0 := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	batch := []Deliverable{
		{ID: "x1", InboxRank: 3, UnblocksCount: 4, Risky: false, VerificationCost: 10, DeliveredAt: t0},
		{ID: "x2", Deps: []string{"x1"}, InboxRank: 1, UnblocksCount: 1, Risky: false, VerificationCost: 2, DeliveredAt: t0},
		{ID: "x3", InboxRank: 3, UnblocksCount: 4, Risky: true, VerificationCost: 10, DeliveredAt: t0},
		{ID: "x4", Deps: []string{"x2"}, InboxRank: 2, UnblocksCount: 8, Risky: false, VerificationCost: 1, DeliveredAt: t0},
		{ID: "x5", InboxRank: 10, UnblocksCount: 0, Risky: false, VerificationCost: 5, DeliveredAt: t0},
	}

	perms := generateDeliverablePermutations(batch)
	if len(perms) != 120 {
		t.Fatalf("expected 120 permutations, got %d", len(perms))
	}

	canonical := OrderDeliverables(perms[0])
	for i := 1; i < len(perms); i++ {
		got := OrderDeliverables(perms[i])
		if !reflect.DeepEqual(got, canonical) {
			t.Fatalf("Permutation %d yielded order %+v, want canonical %+v", i, got, canonical)
		}
	}

	pos := make(map[string]int, len(canonical))
	for idx, item := range canonical {
		pos[item.ID] = idx
	}
	if pos["x1"] >= pos["x2"] {
		t.Fatalf("x1 (pos %d) came after x2 (pos %d)", pos["x1"], pos["x2"])
	}
	if pos["x2"] >= pos["x4"] {
		t.Fatalf("x2 (pos %d) came after x4 (pos %d)", pos["x2"], pos["x4"])
	}
}

func generateDeliverablePermutations(items []Deliverable) [][]Deliverable {
	if len(items) <= 1 {
		return [][]Deliverable{items}
	}
	var res [][]Deliverable
	for i, item := range items {
		rest := make([]Deliverable, 0, len(items)-1)
		rest = append(rest, items[:i]...)
		rest = append(rest, items[i+1:]...)
		for _, sub := range generateDeliverablePermutations(rest) {
			perm := make([]Deliverable, 0, len(items))
			perm = append(perm, item)
			perm = append(perm, sub...)
			res = append(res, perm)
		}
	}
	return res
}
