package inboxrank_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
)

func TestInputs_OrderIsOrderOverTheWholeQueueWithTheRecurrenceCounts(t *testing.T) {
	ready := []inboxbatch.Item{item("plain", weight(0.6)), item("blocker", weight(0.5)), item("recurring", weight(0.5))}
	queue := append(ready, item("waiting", deps("blocker")))
	in := inboxrank.Inputs{Config: defaults(), Now: rankNow, Recurrence: map[string]int{"recurring": 5}}
	got := in.Order(ready, queue)
	counts := func(it inboxbatch.Item) int { return in.Recurrence[it.ID] }
	want := inboxrank.Order(ready, defaults(), inboxrank.Context{Now: rankNow, Queue: queue, Recurrence: counts})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Inputs.Order = %v, want Order with the same inputs %v", ids(got), ids(want))
	}
	if b := got[0].Breakdown.Facts; got[0].Item.ID != "recurring" || b.Recurrence != 5 {
		t.Errorf("the recurrence count lifts its item to the top: %v %+v", ids(got), b)
	}
	if blocker := got[slicesIndex(got, "blocker")].Breakdown.Facts.Unblocks; blocker != 1 {
		t.Errorf("a waiting dependent outside the ordered list still counts: unblocks = %d", blocker)
	}
}

func slicesIndex(ranked []inboxrank.Ranked, id string) int {
	for i, r := range ranked {
		if r.Item.ID == id {
			return i
		}
	}
	return -1
}

func TestSequence_OrdersAnySubsetByTheRankAndKeepsUnrankedItemsLast(t *testing.T) {
	ranked := inboxrank.Order([]inboxbatch.Item{item("low", weight(0.1)), item("high", weight(0.9)), item("mid", weight(0.5))}, defaults(), ctxFor())
	order := inboxrank.Sequence(ranked)
	got := order([]inboxbatch.Item{item("unranked-a"), item("low"), item("unranked-b"), item("high")})
	var gotIDs []string
	for _, it := range got {
		gotIDs = append(gotIDs, it.ID)
	}
	if want := []string{"high", "low", "unranked-a", "unranked-b"}; !reflect.DeepEqual(gotIDs, want) {
		t.Errorf("Sequence = %v, want %v", gotIDs, want)
	}
}

func TestSequence_KeepsSameIDItemsApartByPath(t *testing.T) {
	a := item("dup", weight(0.9), func(it *inboxbatch.Item) { it.Path = "a.json" })
	b := item("dup", weight(0.1), func(it *inboxbatch.Item) { it.Path = "b.json" })
	got := inboxrank.Sequence(inboxrank.Order([]inboxbatch.Item{b, a}, defaults(), ctxFor()))([]inboxbatch.Item{b, a})
	if got[0].Path != "a.json" || got[1].Path != "b.json" {
		t.Errorf("Sequence = %s, %s; want the heavier a.json first", got[0].Path, got[1].Path)
	}
}

func TestLabels_NameTheLargestContributionAndTheEarlierFactorOnATie(t *testing.T) {
	tied := inboxrank.Ranked{Item: item("tied"), Breakdown: inboxrank.Breakdown{Score: 0.8, Terms: []inboxrank.Term{
		{Factor: inboxrank.FactorBase, Contribution: 0.2},
		{Factor: inboxrank.FactorClass, Contribution: 0.3},
		{Factor: inboxrank.FactorUnblocks, Contribution: 0.3},
	}}}
	if got := inboxrank.Labels([]inboxrank.Ranked{tied})(tied.Item); got != "score 0.8000, top factor class" {
		t.Errorf("label = %q, want the larger contribution, and class over unblocks on the tie", got)
	}
}

func TestLabels_ShowEachRankedItemsScoreAndTopFactor(t *testing.T) {
	ranked := inboxrank.Order([]inboxbatch.Item{item("x", weight(1), class("correctness"), filed(""))}, defaults(), ctxFor())
	label := inboxrank.Labels(ranked)
	if got := label(ranked[0].Item); got != "score 0.6500, top factor base" {
		t.Errorf("label = %q", got)
	}
	if got := label(item("absent")); got != "not ranked" {
		t.Errorf("an item the rank never saw says so: %q", got)
	}
}
