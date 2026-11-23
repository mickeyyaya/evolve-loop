package cyclestate

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func snap(gap float64, statuses ...string) RoundSnapshot {
	s := RoundSnapshot{Observed: true, GapMass: gap, Statuses: map[string]string{}}
	for i := 0; i+1 < len(statuses); i += 2 {
		s.Statuses[statuses[i]] = statuses[i+1]
	}
	return s
}

func observed(r *ReviewRepair) []bool {
	var out []bool
	for _, s := range r.Rounds {
		out = append(out, s.Observed)
	}
	return out
}

func gaps(r *ReviewRepair) []float64 {
	var out []float64
	for _, s := range r.Rounds {
		out = append(out, s.GapMass)
	}
	return out
}

func TestReviewRepair_WithRoundAppendsAndTruncatesToTheRound(t *testing.T) {
	t.Parallel()

	first := (*ReviewRepair)(nil).WithRound(1, snap(4, "a", "OPEN"))
	second := first.WithRound(2, snap(3, "a", "FIXED"))
	if !reflect.DeepEqual(gaps(second), []float64{4, 3}) || second.Rounds[0].Statuses["a"] != "OPEN" || second.Rounds[1].Statuses["a"] != "FIXED" {
		t.Fatalf("round 2 history = %+v, want both rounds with their own statuses", second.Rounds)
	}
	if !reflect.DeepEqual(gaps(first), []float64{4}) {
		t.Fatalf("round 1 value mutated to %v, the update must return a new value", gaps(first))
	}
	if again := second.WithRound(2, snap(9)); !reflect.DeepEqual(gaps(again), []float64{4, 9}) {
		t.Fatalf("re-run of round 2 = %v, want [4 9]: a repeated round replaces its entry", gaps(again))
	}
	skipped := first.WithRound(3, snap(1))
	if got := observed(skipped); !reflect.DeepEqual(got, []bool{true, false, true}) {
		t.Fatalf("round 3 after round 1 observed = %v, want [true false true]: a missed round is marked unobserved, not held as a clean round", got)
	}
}

func TestReviewRepair_RoundBelowOneChangesNothing(t *testing.T) {
	t.Parallel()

	state := &ReviewRepair{Rounds: []RoundSnapshot{snap(2)}}
	if got := state.WithRound(0, snap(5)); got != state {
		t.Fatalf("round 0 returned %+v, want the same state", got)
	}
	if got := (*ReviewRepair)(nil).WithRound(0, snap(5)); got != nil {
		t.Fatalf("round 0 on nil returned %+v, want nil", got)
	}
}

func TestReviewRepair_SnapshotsIsACopyAndNilIsEmpty(t *testing.T) {
	t.Parallel()

	if (*ReviewRepair)(nil).Snapshots() != nil {
		t.Fatal("nil state snapshots must be nil")
	}
	state := &ReviewRepair{Rounds: []RoundSnapshot{snap(1), snap(2)}}
	h := state.Snapshots()
	h[0] = snap(99)
	if state.Rounds[0].GapMass != 1 {
		t.Fatal("Snapshots returned the backing slice, want a copy")
	}
}

func TestCycleState_ReviewRepairOmittedWhenAbsentAndRoundTrips(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(CycleState{CycleID: 1})
	if err != nil || strings.Contains(string(raw), "review_repair") {
		t.Fatalf("empty state json = %s (err %v), want no review_repair key", raw, err)
	}
	var old CycleState
	if err := json.Unmarshal([]byte(`{"cycle_id":2}`), &old); err != nil || old.ReviewRepair != nil {
		t.Fatalf("old checkpoint = %+v (err %v), want it to parse with no review state", old, err)
	}
	raw, _ = json.Marshal(CycleState{CycleID: 3, ReviewRepair: &ReviewRepair{Rounds: []RoundSnapshot{snap(1.5, "x", "FIXED")}}})
	var back CycleState
	if err := json.Unmarshal(raw, &back); err != nil || !reflect.DeepEqual(back.ReviewRepair.Rounds, []RoundSnapshot{snap(1.5, "x", "FIXED")}) {
		t.Fatalf("round trip = %+v (err %v)", back.ReviewRepair, err)
	}
}
