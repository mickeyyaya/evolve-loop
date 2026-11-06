package panestream

import (
	"testing"
	"time"
)

func TestSeatHealthAmplify_EveryFactCombinationFollowsTheFrozenRuleOrder(t *testing.T) {
	t.Parallel()
	window := 5 * time.Minute
	progress := []time.Duration{-time.Hour, 0, window, window + 1, 100 * time.Hour}
	for _, alive := range []bool{false, true} {
		for _, open := range []bool{false, true} {
			for _, busy := range []bool{false, true} {
				for _, done := range []bool{false, true} {
					for _, since := range progress {
						facts := SeatFacts{SessionAlive: alive, RunOpen: open, Busy: busy, DeliverableExists: done, SinceProgress: since, StallWindow: window}
						got := ClassifySeat(facts)
						if got != expectedSeat(facts) {
							t.Errorf("ClassifySeat(%+v) = %s, want %s", facts, got, expectedSeat(facts))
						}
						if got.String() == "unknown" {
							t.Errorf("ClassifySeat(%+v) returned the unknown state", facts)
						}
					}
				}
			}
		}
	}
}

func expectedSeat(f SeatFacts) SeatHealth {
	switch {
	case f.DeliverableExists:
		return SeatIdle
	case !f.SessionAlive && f.RunOpen:
		return SeatZombie
	case !f.SessionAlive:
		return SeatIdle
	case !f.Busy:
		return SeatBlocked
	case f.SinceProgress > f.StallWindow:
		return SeatStalled
	default:
		return SeatWorking
	}
}

func TestSeatHealthAmplify_ZeroValueFactsReadIdleNotZombie(t *testing.T) {
	t.Parallel()
	if got := ClassifySeat(SeatFacts{}); got != SeatIdle {
		t.Errorf("ClassifySeat(zero) = %s, want idle", got)
	}
}

func TestSeatHealthAmplify_ANegativeSinceProgressNeverReadsStalled(t *testing.T) {
	t.Parallel()
	facts := SeatFacts{SessionAlive: true, RunOpen: true, Busy: true, SinceProgress: -time.Hour, StallWindow: time.Minute}
	if got := ClassifySeat(facts); got != SeatWorking {
		t.Errorf("ClassifySeat(%+v) = %s, want working", facts, got)
	}
}

func TestSeatHealthAmplify_AZeroStallWindowStallsAnyPositiveSilence(t *testing.T) {
	t.Parallel()
	base := SeatFacts{SessionAlive: true, RunOpen: true, Busy: true}
	if got := ClassifySeat(base); got != SeatWorking {
		t.Errorf("no silence and no window = %s, want working", got)
	}
	base.SinceProgress = time.Nanosecond
	if got := ClassifySeat(base); got != SeatStalled {
		t.Errorf("one tick of silence and no window = %s, want stalled", got)
	}
}

func TestSeatHealthAmplify_StringOfAnUndefinedValueReadsUnknown(t *testing.T) {
	t.Parallel()
	for _, v := range []SeatHealth{SeatHealth(-1), SeatHealth(99), SeatHealth(1 << 20)} {
		if got := v.String(); got != "unknown" {
			t.Errorf("SeatHealth(%d).String() = %q, want unknown", int(v), got)
		}
	}
}

func TestSeatHealthAmplify_ClassifySeatIsPureAndDeterministic(t *testing.T) {
	t.Parallel()
	facts := SeatFacts{SessionAlive: true, RunOpen: true, Busy: true, SinceProgress: time.Hour, StallWindow: time.Minute}
	first := ClassifySeat(facts)
	for i := 0; i < 50; i++ {
		if got := ClassifySeat(facts); got != first {
			t.Fatalf("call %d = %s, first = %s", i, got, first)
		}
	}
}
