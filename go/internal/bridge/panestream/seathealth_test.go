package panestream

import (
	"testing"
	"time"
)

func TestSeatHealth_ClassifySeatNamesOneStatePerFactSet(t *testing.T) {
	t.Parallel()
	window := 10 * time.Minute
	cases := []struct {
		name  string
		facts SeatFacts
		want  SeatHealth
	}{
		{"a pane at its prompt with no deliverable is blocked", SeatFacts{SessionAlive: true, RunOpen: true, Busy: false, StallWindow: window}, SeatBlocked},
		{"a dead session with an open run is a zombie", SeatFacts{SessionAlive: false, RunOpen: true, Busy: true, StallWindow: window}, SeatZombie},
		{"a dead session at its prompt with an open run is a zombie, not blocked", SeatFacts{SessionAlive: false, RunOpen: true, Busy: false, StallWindow: window}, SeatZombie},
		{"a dead session whose run closed is idle", SeatFacts{SessionAlive: false, RunOpen: false, StallWindow: window}, SeatIdle},
		{"a deliverable makes a live seat idle", SeatFacts{SessionAlive: true, RunOpen: true, DeliverableExists: true, Busy: false, StallWindow: window}, SeatIdle},
		{"a deliverable outranks a dead session with an open run", SeatFacts{SessionAlive: false, RunOpen: true, DeliverableExists: true, StallWindow: window}, SeatIdle},
		{"a deliverable outranks a stalled pane", SeatFacts{SessionAlive: true, RunOpen: true, DeliverableExists: true, Busy: true, SinceProgress: 3 * window, StallWindow: window}, SeatIdle},
		{"a busy pane with fresh output is working", SeatFacts{SessionAlive: true, RunOpen: true, Busy: true, SinceProgress: time.Second, StallWindow: window}, SeatWorking},
		{"output exactly at the window is still working", SeatFacts{SessionAlive: true, RunOpen: true, Busy: true, SinceProgress: window, StallWindow: window}, SeatWorking},
		{"output one tick past the window is stalled", SeatFacts{SessionAlive: true, RunOpen: true, Busy: true, SinceProgress: window + time.Nanosecond, StallWindow: window}, SeatStalled},
		{"a long silent busy pane is stalled", SeatFacts{SessionAlive: true, RunOpen: true, Busy: true, SinceProgress: 3 * time.Hour, StallWindow: window}, SeatStalled},
		{"a silent pane at its prompt is blocked, not stalled", SeatFacts{SessionAlive: true, RunOpen: true, Busy: false, SinceProgress: 3 * time.Hour, StallWindow: window}, SeatBlocked},
	}
	for _, c := range cases {
		if got := ClassifySeat(c.facts); got != c.want {
			t.Errorf("%s: ClassifySeat(%+v) = %s, want %s", c.name, c.facts, got, c.want)
		}
	}
}

func TestSeatHealth_StringNamesEachStateAndNoOtherValue(t *testing.T) {
	t.Parallel()
	want := map[SeatHealth]string{
		SeatWorking: "working",
		SeatBlocked: "blocked",
		SeatStalled: "stalled",
		SeatZombie:  "zombie",
		SeatIdle:    "idle",
	}
	for state, name := range want {
		if got := state.String(); got != name {
			t.Errorf("SeatHealth(%d).String() = %q, want %q", int(state), got, name)
		}
	}
	if got := SeatHealth(0).String(); got != "unknown" {
		t.Errorf("the zero SeatHealth reads %q, want unknown", got)
	}
	seen := map[string]bool{}
	for _, name := range want {
		if seen[name] {
			t.Errorf("state name %q repeats", name)
		}
		seen[name] = true
	}
}
