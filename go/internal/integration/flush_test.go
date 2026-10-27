package integration

import (
	"slices"
	"testing"
	"time"
)

var decisionTime = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

func youngDeliverable(id string) Deliverable {
	return Deliverable{ID: id, DeliveredAt: decisionTime.Add(-time.Minute)}
}

func agedDeliverable(id string, age time.Duration) Deliverable {
	return Deliverable{ID: id, DeliveredAt: decisionTime.Add(-age)}
}

func unblockingDeliverable(id string) Deliverable {
	d := youngDeliverable(id)
	d.UnblocksQueuedWork = true
	return d
}

func riskyDeliverable(id string) Deliverable {
	d := youngDeliverable(id)
	d.Risky = true
	return d
}

func idsOf(batch []Deliverable) []string {
	ids := make([]string, 0, len(batch))
	for _, d := range batch {
		ids = append(ids, d.ID)
	}
	return ids
}

func assertIDs(t *testing.T, label string, got []Deliverable, want ...string) {
	t.Helper()
	if !slices.Equal(idsOf(got), want) && !(len(got) == 0 && len(want) == 0) {
		t.Errorf("%s = %v, want %v", label, idsOf(got), want)
	}
}

func TestDecideFlushEachD7TriggerFlushesTheWholeBatchOnce(t *testing.T) {
	cases := []struct {
		name      string
		batch     []Deliverable
		waveEnded bool
		policy    FlushPolicy
		trigger   FlushTrigger
	}{
		{"the wave ends", []Deliverable{youngDeliverable("a")}, true, DefaultFlushPolicy(), TriggerWaveEnd},
		{"the batch reaches the default cap", []Deliverable{youngDeliverable("a"), youngDeliverable("b"), youngDeliverable("c"), youngDeliverable("d")}, false, DefaultFlushPolicy(), TriggerCapReached},
		{"the batch reaches an adapted cap", []Deliverable{youngDeliverable("a"), youngDeliverable("b")}, false, FlushPolicy{Cap: 2, MaxAgeMinutes: 45}, TriggerCapReached},
		{"the oldest deliverable reaches max age", []Deliverable{youngDeliverable("a"), agedDeliverable("b", 45*time.Minute)}, false, DefaultFlushPolicy(), TriggerMaxAge},
		{"a deliverable unblocks queued work", []Deliverable{youngDeliverable("a"), unblockingDeliverable("b")}, false, DefaultFlushPolicy(), TriggerUnblocksQueuedWork},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			decision := DecideFlush(c.batch, c.waveEnded, c.policy, decisionTime)
			if decision.Trigger != c.trigger {
				t.Errorf("trigger = %q, want %q", decision.Trigger, c.trigger)
			}
			assertIDs(t, "flushed", decision.Flush, idsOf(c.batch)...)
			assertIDs(t, "held", decision.Hold)
		})
	}
}

func TestDecideFlushMaxAgeBoundaryUsesTheOldestDeliverableAnywhereInTheBatch(t *testing.T) {
	cases := []struct {
		name    string
		oldest  time.Duration
		policy  FlushPolicy
		trigger FlushTrigger
	}{
		{"one second before max age", 45*time.Minute - time.Second, DefaultFlushPolicy(), TriggerNone},
		{"exactly max age", 45 * time.Minute, DefaultFlushPolicy(), TriggerMaxAge},
		{"past max age", 46 * time.Minute, DefaultFlushPolicy(), TriggerMaxAge},
		{"a shorter configured max age", 10 * time.Minute, FlushPolicy{Cap: 4, MaxAgeMinutes: 10}, TriggerMaxAge},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			batch := []Deliverable{youngDeliverable("a"), agedDeliverable("oldest", c.oldest), youngDeliverable("c")}
			decision := DecideFlush(batch, false, c.policy, decisionTime)
			if decision.Trigger != c.trigger {
				t.Errorf("trigger = %q, want %q", decision.Trigger, c.trigger)
			}
		})
	}
}

func TestDecideFlushFirstTriggerInD7OrderNamesTheDecision(t *testing.T) {
	fullAndOldAndUnblocking := []Deliverable{
		agedDeliverable("a", 50*time.Minute),
		unblockingDeliverable("b"),
		youngDeliverable("c"),
		youngDeliverable("d"),
	}
	cases := []struct {
		name      string
		batch     []Deliverable
		waveEnded bool
		trigger   FlushTrigger
	}{
		{"wave end beats cap, age and unblock", fullAndOldAndUnblocking, true, TriggerWaveEnd},
		{"cap beats age and unblock", fullAndOldAndUnblocking, false, TriggerCapReached},
		{"age beats unblock", fullAndOldAndUnblocking[:2], false, TriggerMaxAge},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			decision := DecideFlush(c.batch, c.waveEnded, DefaultFlushPolicy(), decisionTime)
			if decision.Trigger != c.trigger {
				t.Errorf("trigger = %q, want %q", decision.Trigger, c.trigger)
			}
			assertIDs(t, "flushed", decision.Flush, idsOf(c.batch)...)
		})
	}
}

func TestDecideFlushBelowCapWithNoTriggerHoldsTheBatch(t *testing.T) {
	batch := []Deliverable{youngDeliverable("a"), youngDeliverable("b"), youngDeliverable("c")}
	decision := DecideFlush(batch, false, DefaultFlushPolicy(), decisionTime)
	if decision.Trigger != TriggerNone {
		t.Errorf("trigger = %q, want %q", decision.Trigger, TriggerNone)
	}
	assertIDs(t, "flushed", decision.Flush)
	assertIDs(t, "held", decision.Hold, "a", "b", "c")
}

func TestDecideFlushEmptyBatchNeverFlushesEvenWhenTheWaveEnds(t *testing.T) {
	for _, batch := range [][]Deliverable{nil, {}} {
		decision := DecideFlush(batch, true, DefaultFlushPolicy(), decisionTime)
		if decision.Trigger != TriggerNone || len(decision.Flush) != 0 || len(decision.Hold) != 0 {
			t.Errorf("empty batch decision = %+v, want no trigger, nothing flushed, nothing held", decision)
		}
	}
}

func TestDecideFlushRiskyDeliverableFlushesAlone(t *testing.T) {
	batch := []Deliverable{youngDeliverable("a"), riskyDeliverable("risky"), youngDeliverable("c")}
	decision := DecideFlush(batch, false, DefaultFlushPolicy(), decisionTime)
	if decision.Trigger != TriggerRiskyAlone {
		t.Errorf("trigger = %q, want %q", decision.Trigger, TriggerRiskyAlone)
	}
	assertIDs(t, "flushed", decision.Flush, "risky")
	assertIDs(t, "held", decision.Hold, "a", "c")
}

func TestDecideFlushRiskyDeliverableStaysAloneWhenEveryOtherTriggerHolds(t *testing.T) {
	batch := []Deliverable{
		agedDeliverable("old", 90*time.Minute),
		unblockingDeliverable("unblocks"),
		riskyDeliverable("risky"),
		youngDeliverable("d"),
	}
	decision := DecideFlush(batch, true, DefaultFlushPolicy(), decisionTime)
	if decision.Trigger != TriggerRiskyAlone {
		t.Errorf("trigger = %q, want %q", decision.Trigger, TriggerRiskyAlone)
	}
	assertIDs(t, "flushed", decision.Flush, "risky")
	assertIDs(t, "held", decision.Hold, "old", "unblocks", "d")
}

func TestDecideFlushOnlyRiskyDeliverableLeavesNothingHeld(t *testing.T) {
	decision := DecideFlush([]Deliverable{riskyDeliverable("risky")}, false, DefaultFlushPolicy(), decisionTime)
	if decision.Trigger != TriggerRiskyAlone {
		t.Errorf("trigger = %q, want %q", decision.Trigger, TriggerRiskyAlone)
	}
	assertIDs(t, "flushed", decision.Flush, "risky")
	assertIDs(t, "held", decision.Hold)
}

func TestDecideFlushTwoRiskyDeliverablesFlushOneAtATime(t *testing.T) {
	batch := []Deliverable{riskyDeliverable("first"), youngDeliverable("b"), riskyDeliverable("second")}
	first := DecideFlush(batch, false, DefaultFlushPolicy(), decisionTime)
	assertIDs(t, "first flushed", first.Flush, "first")
	assertIDs(t, "first held", first.Hold, "b", "second")

	second := DecideFlush(first.Hold, false, DefaultFlushPolicy(), decisionTime)
	if second.Trigger != TriggerRiskyAlone {
		t.Errorf("second trigger = %q, want %q", second.Trigger, TriggerRiskyAlone)
	}
	assertIDs(t, "second flushed", second.Flush, "second")
	assertIDs(t, "second held", second.Hold, "b")
}

func TestDecideFlushHeldRemainderFlushesOnTheNextTrigger(t *testing.T) {
	batch := []Deliverable{youngDeliverable("a"), riskyDeliverable("risky")}
	first := DecideFlush(batch, true, DefaultFlushPolicy(), decisionTime)
	next := DecideFlush(first.Hold, true, DefaultFlushPolicy(), decisionTime)
	if next.Trigger != TriggerWaveEnd {
		t.Errorf("next trigger = %q, want %q", next.Trigger, TriggerWaveEnd)
	}
	assertIDs(t, "next flushed", next.Flush, "a")
}

func TestDecideFlushLeavesTheInputBatchUnchanged(t *testing.T) {
	batch := []Deliverable{youngDeliverable("a"), riskyDeliverable("risky"), youngDeliverable("c")}
	before := slices.Clone(batch)
	decision := DecideFlush(batch, true, DefaultFlushPolicy(), decisionTime)
	if len(decision.Flush) > 0 {
		decision.Flush[0].ID = "mutated"
	}
	if len(decision.Hold) > 0 {
		decision.Hold[0].ID = "mutated"
	}
	if !slices.Equal(batch, before) {
		t.Errorf("input batch = %v after the decision, want %v", batch, before)
	}
}
