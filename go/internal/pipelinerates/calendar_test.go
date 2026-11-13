package pipelinerates

import (
	"testing"
	"time"
)

func deliveriesOn(day string, n int) []Cycle {
	at, _ := time.Parse(time.RFC3339, day+"T12:00:00Z")
	out := make([]Cycle, n)
	for i := range out {
		out[i] = Cycle{ID: i + 1, Wave: 1, Shipped: true, ShippedAt: at}
	}
	return out
}

func f21Fixture() []Cycle {
	var cs []Cycle
	for _, d := range []struct {
		day string
		n   int
	}{{"2026-11-09", 18}, {"2026-11-10", 14}, {"2026-11-11", 22}, {"2026-11-12", 4}, {"2026-11-13", 5}} {
		cs = append(cs, deliveriesOn(d.day, d.n)...)
	}
	return cs
}

func TestCalendarThroughput_CountsDeliveriesForEachDay(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 13, 17, 30, 0, 0, time.UTC)

	readings := CalendarThroughput(f21Fixture(), now)

	want := map[string]float64{"day:2026-11-07": 0, "day:2026-11-08": 0, "day:2026-11-09": 18, "day:2026-11-10": 14, "day:2026-11-11": 22, "day:2026-11-12": 4, "day:2026-11-13": 5}
	if len(readings) != len(want) {
		t.Fatalf("readings = %d, want %d (the last 7 days)", len(readings), len(want))
	}
	for _, r := range readings {
		v, ok := want[r.Window]
		if !ok || r.Index != "PQ6" || r.Interval.Value != v || r.N != 1 || r.Threshold != nil || r.Status != "" {
			t.Errorf("reading %+v, want PQ6 %v for the day, one observation, no threshold", r, v)
		}
	}
}

func TestCalendarThroughput_LeavesOutACycleThatDidNotShipOrHasNoShipTime(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 13, 17, 30, 0, 0, time.UTC)
	at := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	old := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	cs := []Cycle{{Shipped: true, ShippedAt: at}, {Shipped: false, ShippedAt: at}, {Shipped: true}, {Shipped: true, ShippedAt: old}}

	readings := CalendarThroughput(cs, now)

	last := readings[len(readings)-1]
	if last.Window != "day:2026-11-13" || last.Interval.Value != 1 {
		t.Errorf("last reading %+v, want 1 delivery on 11-13: a non-ship, an undated ship and a ship before the window do not count", last)
	}
}

func TestCalendarThroughput_BucketsByTheLocalDayOfNow(t *testing.T) {
	t.Parallel()
	jst := time.FixedZone("JST", 9*3600)
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, jst)
	shipped := time.Date(2026, 11, 12, 20, 0, 0, 0, time.UTC)

	readings := CalendarThroughput([]Cycle{{Shipped: true, ShippedAt: shipped}}, now)

	last := readings[len(readings)-1]
	if last.Window != "day:2026-11-13" || last.Interval.Value != 1 {
		t.Errorf("last reading %+v, want the 20:00Z ship on the JST day 11-13", last)
	}
}
