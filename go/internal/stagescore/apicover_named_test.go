package stagescore

import "testing"

func TestAPICoverNamedExports(t *testing.T) {
	t.Parallel()
	records := []Record{
		{Decision: WouldStop, Outcome: Passed},
		{Decision: WouldSkip, Outcome: Red},
		{Decision: WouldHold, LaneRouted: true},
	}
	var want Tally = Tally{Decisions: 3, FalseStops: 1, Escapes: 1, Conflicts: 1}
	got, err := Score(records)
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if got != want {
		t.Fatalf("Score = %+v, want %+v", got, want)
	}
	var d Decision = WouldStop
	var o Outcome = Red
	if d == "" || o == "" {
		t.Fatal("empty constants")
	}
}
