package panestream

import (
	"math"
	"testing"
	"time"
)

func TestSeatHealthExtremes_AMaximalSilenceAgainstAMaximalWindowIsNotStalled(t *testing.T) {
	t.Parallel()
	facts := SeatFacts{SessionAlive: true, RunOpen: true, Busy: true, SinceProgress: time.Duration(math.MaxInt64), StallWindow: time.Duration(math.MaxInt64)}
	if got := ClassifySeat(facts); got != SeatWorking {
		t.Errorf("ClassifySeat(%+v) = %s, want working", facts, got)
	}
}

func TestSeatHealthExtremes_AMaximalSilenceAgainstAShortWindowIsStalled(t *testing.T) {
	t.Parallel()
	facts := SeatFacts{SessionAlive: true, RunOpen: true, Busy: true, SinceProgress: time.Duration(math.MaxInt64), StallWindow: time.Second}
	if got := ClassifySeat(facts); got != SeatStalled {
		t.Errorf("ClassifySeat(%+v) = %s, want stalled", facts, got)
	}
}

func TestSeatHealthExtremes_ADeadSessionIgnoresBusyAndSilence(t *testing.T) {
	t.Parallel()
	for _, busy := range []bool{false, true} {
		for _, since := range []time.Duration{-time.Hour, 0, time.Duration(math.MaxInt64)} {
			zombie := SeatFacts{SessionAlive: false, RunOpen: true, Busy: busy, SinceProgress: since, StallWindow: time.Minute}
			if got := ClassifySeat(zombie); got != SeatZombie {
				t.Errorf("ClassifySeat(%+v) = %s, want zombie", zombie, got)
			}
			gone := SeatFacts{SessionAlive: false, RunOpen: false, Busy: busy, SinceProgress: since, StallWindow: time.Minute}
			if got := ClassifySeat(gone); got != SeatIdle {
				t.Errorf("ClassifySeat(%+v) = %s, want idle", gone, got)
			}
		}
	}
}

func TestSeatHealthExtremes_ANegativeWindowStallsAnyNonNegativeSilenceOfABusyPane(t *testing.T) {
	t.Parallel()
	facts := SeatFacts{SessionAlive: true, RunOpen: true, Busy: true, SinceProgress: 0, StallWindow: -time.Second}
	if got := ClassifySeat(facts); got != SeatStalled {
		t.Errorf("ClassifySeat(%+v) = %s, want stalled", facts, got)
	}
}

func TestSeatHealthExtremes_ALivePaneWithAClosedRunStillReadsByPromptAndSilence(t *testing.T) {
	t.Parallel()
	prompt := SeatFacts{SessionAlive: true, RunOpen: false, Busy: false, StallWindow: time.Minute}
	if got := ClassifySeat(prompt); got != SeatBlocked {
		t.Errorf("ClassifySeat(%+v) = %s, want blocked", prompt, got)
	}
}
