package fleet

import (
	"reflect"
	"testing"
)

func pqContains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func TestNewPrefixQueue_WindowStartsAtThree(t *testing.T) {
	q := NewPrefixQueue()
	if q == nil {
		t.Fatal("NewPrefixQueue returned nil")
	}
	if got := q.Window(); got != 3 {
		t.Errorf("initial Window() = %d, want 3", got)
	}
}

func TestPrefixQueue_AIMDWindow(t *testing.T) {
	q := NewPrefixQueue()
	q.OnGreen()
	q.OnGreen()
	if got := q.Window(); got != 5 {
		t.Errorf("Window after 2 greens = %d, want 5", got)
	}
	q.OnRed()
	if got := q.Window(); got != 2 {
		t.Errorf("Window after red = %d, want 2", got)
	}
	q.OnRed()
	q.OnRed()
	if got := q.Window(); got != 1 {
		t.Errorf("Window at floor = %d, want 1", got)
	}
}

func TestPrefixQueue_ComposePrefixes(t *testing.T) {
	q := NewPrefixQueue()
	q.Enqueue(LaneCandidate{ID: "L1", Tier: TierRollup, Files: []string{"a/a.go"}})
	q.Enqueue(LaneCandidate{ID: "L2", Tier: TierMaybe, Files: []string{"b/b.go"}})
	q.Enqueue(LaneCandidate{ID: "IFFY", Tier: TierIffy, Files: []string{"core/c.go"}})

	got := q.ComposePrefixes()
	want := [][]string{{"L1"}, {"L1", "L2"}, {"IFFY"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ComposePrefixes() = %v, want %v", got, want)
	}
	for _, p := range got {
		if pqContains(p, "IFFY") && len(p) != 1 {
			t.Errorf("iffy lane must be solo, found in %v", p)
		}
	}
}

func TestPrefixQueue_ResolveCulprit(t *testing.T) {
	q := NewPrefixQueue()
	q.Enqueue(LaneCandidate{ID: "L1", Tier: TierMaybe, Files: []string{"a/a.go"}})
	q.Enqueue(LaneCandidate{ID: "L2", Tier: TierMaybe, Files: []string{"b/b.go"}})
	q.Enqueue(LaneCandidate{ID: "L3", Tier: TierMaybe, Files: []string{"c/c.go"}})

	calls := 0
	landed, ejected := q.ResolveCulprit(func(ids []string) bool {
		calls++
		return !pqContains(ids, "L2")
	})
	if !pqContains(landed, "L1") || !pqContains(landed, "L3") || pqContains(landed, "L2") {
		t.Errorf("landed = %v, want L1,L3 (not L2)", landed)
	}
	if len(ejected) != 1 || ejected[0] != "L2" {
		t.Errorf("ejected = %v, want [L2]", ejected)
	}
	if calls > 6 {
		t.Errorf("verify called %d times, want linear NNFI (<=6)", calls)
	}
}

func TestPrefixQueueType_ZeroValue(t *testing.T) {
	var q PrefixQueue
	var tier RiskTier = TierMaybe
	q.Enqueue(LaneCandidate{ID: "x", Tier: tier, Files: []string{"x/x.go"}})
	if got := q.ComposePrefixes(); !reflect.DeepEqual(got, [][]string{{"x"}}) {
		t.Errorf("zero-value ComposePrefixes() = %v, want [[x]]", got)
	}
}

func TestLandingMode_Vocabulary(t *testing.T) {
	if DefaultLandingMode() != LandingPerLane {
		t.Errorf("DefaultLandingMode() = %q, want %q", DefaultLandingMode(), LandingPerLane)
	}
	for _, m := range []LandingMode{LandingPerLane, LandingPrefixQueue} {
		got, err := ParseLandingMode(string(m))
		if err != nil || got != m {
			t.Errorf("ParseLandingMode(%q) = (%q,%v), want (%q,nil)", m, got, err, m)
		}
	}
	if _, err := ParseLandingMode("bogus"); err == nil {
		t.Error("ParseLandingMode(\"bogus\") = nil error, want a validation error")
	}
}
