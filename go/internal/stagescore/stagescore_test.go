package stagescore

import "testing"

func TestScorerCountsFalseStopEscapeConflict(t *testing.T) {
	t.Parallel()
	records := []Record{
		{Decision: WouldStop, Outcome: Passed},
		{Decision: WouldStop, Outcome: Red},
		{Decision: WouldSkip, Outcome: Red},
		{Decision: WouldSkip, Outcome: Passed},
		{Decision: WouldHold, LaneRouted: true},
		{Decision: WouldHold},
	}
	got, err := Score(records)
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	want := Tally{Decisions: 6, FalseStops: 1, Escapes: 1, Conflicts: 1}
	if got != want {
		t.Fatalf("a PASS after would_stop is a false stop, a red after would_skip is an escape, a would_hold on a lane-routed item is a conflict, and nothing else counts; got %+v, want %+v", got, want)
	}
}

func TestScorerCountsEachKindIndependently(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		record Record
		want   Tally
	}{
		"a stop that was right":      {Record{Decision: WouldStop, Outcome: Red}, Tally{Decisions: 1}},
		"a skip that was right":      {Record{Decision: WouldSkip, Outcome: Passed}, Tally{Decisions: 1}},
		"a hold on a free item":      {Record{Decision: WouldHold}, Tally{Decisions: 1}},
		"a false stop":               {Record{Decision: WouldStop, Outcome: Passed}, Tally{Decisions: 1, FalseStops: 1}},
		"an escape":                  {Record{Decision: WouldSkip, Outcome: Red}, Tally{Decisions: 1, Escapes: 1}},
		"a conflict":                 {Record{Decision: WouldHold, LaneRouted: true}, Tally{Decisions: 1, Conflicts: 1}},
		"a lane flag on a stop":      {Record{Decision: WouldStop, Outcome: Red, LaneRouted: true}, Tally{Decisions: 1}},
		"a red outcome on a hold":    {Record{Decision: WouldHold, Outcome: Red}, Tally{Decisions: 1}},
		"a passed outcome on a hold": {Record{Decision: WouldHold, Outcome: Passed}, Tally{Decisions: 1}},
	}
	for name, c := range cases {
		got, err := Score([]Record{c.record})
		if err != nil || got != c.want {
			t.Errorf("%s: got %+v err=%v, want %+v", name, got, err, c.want)
		}
	}
}

func TestScorerRefusesADecisionItDoesNotKnow(t *testing.T) {
	t.Parallel()
	if _, err := Score([]Record{{Decision: WouldStop, Outcome: Passed}, {Decision: "would_sleep", Outcome: Passed}}); err == nil {
		t.Fatal("an unknown decision kind must stop the score, never be skipped as zero")
	}
}

func TestScorerOfNoRecordsIsTheZeroTally(t *testing.T) {
	t.Parallel()
	got, err := Score(nil)
	if err != nil || got != (Tally{}) {
		t.Fatalf("no evidence scores nothing: got %+v err=%v", got, err)
	}
}
