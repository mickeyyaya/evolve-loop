package integration

import (
	"testing"
	"time"
)

func TestOrderDeliverablesEmpty(t *testing.T) {
	if got := OrderDeliverables(nil); len(got) != 0 {
		t.Fatalf("OrderDeliverables(nil) = %v, want empty", got)
	}
	if got := OrderDeliverables([]Deliverable{}); len(got) != 0 {
		t.Fatalf("OrderDeliverables([]) = %v, want empty", got)
	}
}

func TestOrderDeliverablesDirectComparator(t *testing.T) {
	t0 := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)

	if c := compareDeliverables(Deliverable{InboxRank: 1}, Deliverable{InboxRank: 2}); c >= 0 {
		t.Fatalf("rank 1 vs 2 = %d, want < 0", c)
	}
	if c := compareDeliverables(Deliverable{InboxRank: 2}, Deliverable{InboxRank: 1}); c <= 0 {
		t.Fatalf("rank 2 vs 1 = %d, want > 0", c)
	}

	if c := compareDeliverables(Deliverable{UnblocksCount: 5}, Deliverable{UnblocksCount: 2}); c >= 0 {
		t.Fatalf("unblocks 5 vs 2 = %d, want < 0", c)
	}
	if c := compareDeliverables(Deliverable{UnblocksCount: 2}, Deliverable{UnblocksCount: 5}); c <= 0 {
		t.Fatalf("unblocks 2 vs 5 = %d, want > 0", c)
	}

	if c := compareDeliverables(Deliverable{Risky: false}, Deliverable{Risky: true}); c >= 0 {
		t.Fatalf("safe vs risky = %d, want < 0", c)
	}
	if c := compareDeliverables(Deliverable{Risky: true}, Deliverable{Risky: false}); c <= 0 {
		t.Fatalf("risky vs safe = %d, want > 0", c)
	}

	if c := compareDeliverables(Deliverable{VerificationCost: 10}, Deliverable{VerificationCost: 20}); c >= 0 {
		t.Fatalf("cost 10 vs 20 = %d, want < 0", c)
	}
	if c := compareDeliverables(Deliverable{VerificationCost: 20}, Deliverable{VerificationCost: 10}); c <= 0 {
		t.Fatalf("cost 20 vs 10 = %d, want > 0", c)
	}

	if c := compareDeliverables(Deliverable{DeliveredAt: t0}, Deliverable{DeliveredAt: t1}); c >= 0 {
		t.Fatalf("delivered t0 vs t1 = %d, want < 0", c)
	}
	if c := compareDeliverables(Deliverable{DeliveredAt: t1}, Deliverable{DeliveredAt: t0}); c <= 0 {
		t.Fatalf("delivered t1 vs t0 = %d, want > 0", c)
	}

	if c := compareDeliverables(Deliverable{ID: "a"}, Deliverable{ID: "b"}); c >= 0 {
		t.Fatalf("id a vs b = %d, want < 0", c)
	}
	if c := compareDeliverables(Deliverable{ID: "b"}, Deliverable{ID: "a"}); c <= 0 {
		t.Fatalf("id b vs a = %d, want > 0", c)
	}
	if c := compareDeliverables(Deliverable{ID: "a"}, Deliverable{ID: "a"}); c != 0 {
		t.Fatalf("id a vs a = %d, want 0", c)
	}
}

func TestOrderDeliverablesDependenciesAndCycles(t *testing.T) {
	d1 := Deliverable{ID: "consumer", InboxRank: 1, Deps: []string{"producer", "consumer", "unknown"}}
	d2 := Deliverable{ID: "producer", InboxRank: 9}
	ordered := OrderDeliverables([]Deliverable{d1, d2})
	if len(ordered) != 2 || ordered[0].ID != "producer" || ordered[1].ID != "consumer" {
		t.Fatalf("ordered = %v, want producer before consumer", ordered)
	}

	cycleA := Deliverable{ID: "ca", InboxRank: 1, Deps: []string{"cb"}}
	cycleB := Deliverable{ID: "cb", InboxRank: 2, Deps: []string{"ca"}}
	cycleOrdered := OrderDeliverables([]Deliverable{cycleA, cycleB})
	if len(cycleOrdered) != 2 {
		t.Fatalf("cycleOrdered len = %d, want 2", len(cycleOrdered))
	}
}

func TestOrderDeliverablesPurity(t *testing.T) {
	d := Deliverable{ID: "test", Deps: []string{"dep"}}
	batch := []Deliverable{d}
	ordered := OrderDeliverables(batch)
	ordered[0].ID = "mutated"
	ordered[0].Deps[0] = "mutated"
	if batch[0].ID != "test" || batch[0].Deps[0] != "dep" {
		t.Fatalf("input mutated: %v", batch)
	}
}

func ratchetListsRecord(pr int) DeliverableRecord {
	return DeliverableRecord{Source: SourceConsolePR, PR: pr, Audit: AuditBinding{Class: RatchetListsClass}}
}

func TestOrderDeliverablesComposesTheRatchetListsPRLast(t *testing.T) {
	t0 := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	lane := DeliverableRecord{Cycle: 9, InboxRank: 5, DeliveredAt: t0}
	console := DeliverableRecord{Source: SourceConsolePR, PR: 90}
	ratchet := ratchetListsRecord(12)
	byID := map[string]DeliverableRecord{}
	items := []Deliverable{}
	for _, r := range []DeliverableRecord{ratchet, console, lane} {
		byID[r.Deliverable().ID] = r
		items = append(items, r.Deliverable())
	}

	got := OrderDeliverables(items)

	if len(got) != 3 || got[2].ID != "pr-12" {
		t.Fatalf("order = %v, want the ratchet lists PR last", got)
	}
}

func TestOrderDeliverablesTailKeyBeatsEveryOtherKey(t *testing.T) {
	tail := Deliverable{ID: "a", Tail: true}
	other := Deliverable{ID: "b", InboxRank: 9, Risky: true, VerificationCost: 99, DeliveredAt: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)}
	if c := compareDeliverables(tail, other); c <= 0 {
		t.Fatalf("tail vs other = %d, want > 0", c)
	}
	if c := compareDeliverables(other, tail); c >= 0 {
		t.Fatalf("other vs tail = %d, want < 0", c)
	}
	if c := compareDeliverables(tail, Deliverable{ID: "a", Tail: true}); c != 0 {
		t.Fatalf("tail vs tail = %d, want 0", c)
	}
}

func TestDeliverableTailsOnlyTheRatchetListsClass(t *testing.T) {
	if !ratchetListsRecord(3).Deliverable().Tail {
		t.Error("a ratchet-lists record must tail")
	}
	docs := DeliverableRecord{Source: SourceConsolePR, PR: 4, Audit: AuditBinding{Class: "docs-review"}}
	if docs.Deliverable().Tail {
		t.Error("a docs-review record must not tail")
	}
}
