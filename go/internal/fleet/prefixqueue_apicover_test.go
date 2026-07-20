package fleet

import (
	"reflect"
	"testing"
)

// prefixqueue_apicover_test.go — default-tag public-API coverage for the salvaged
// prefix composer (ADR-0069: the acs-tagged go/acs/cycle975+981 predicates do NOT
// run under `go test ./internal/...`, so repo-wide apicover flags every
// prefixqueue.go export as uncovered — the exact gap that reds main). Each test
// names and exercises a real contract (Rule 9), it is not a bare reference.

// pqContains reports whether ids includes id (test-local helper).
func pqContains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// TestNewPrefixQueue_WindowStartsAtThree names NewPrefixQueue + Window and pins
// the AIMD window's compiled start value.
func TestNewPrefixQueue_WindowStartsAtThree(t *testing.T) {
	q := NewPrefixQueue()
	if q == nil {
		t.Fatal("NewPrefixQueue returned nil")
	}
	if got := q.Window(); got != 3 {
		t.Errorf("initial Window() = %d, want 3", got)
	}
}

// TestPrefixQueue_AIMDWindow names OnGreen/OnRed and pins additive-increase,
// multiplicative-decrease, and the floor of 1.
func TestPrefixQueue_AIMDWindow(t *testing.T) {
	q := NewPrefixQueue()
	q.OnGreen()
	q.OnGreen() // 3 -> 5
	if got := q.Window(); got != 5 {
		t.Errorf("Window after 2 greens = %d, want 5", got)
	}
	q.OnRed() // 5 -> 2
	if got := q.Window(); got != 2 {
		t.Errorf("Window after red = %d, want 2", got)
	}
	q.OnRed() // 2 -> 1
	q.OnRed() // floor
	if got := q.Window(); got != 1 {
		t.Errorf("Window at floor = %d, want 1", got)
	}
}

// TestPrefixQueue_ComposePrefixes names Enqueue/ComposePrefixes + LaneCandidate,
// RiskTier and its consts, pinning cumulative composition and solo-slot isolation.
func TestPrefixQueue_ComposePrefixes(t *testing.T) {
	q := NewPrefixQueue()
	q.Enqueue(LaneCandidate{ID: "L1", Tier: TierRollup, Files: []string{"a/a.go"}})
	q.Enqueue(LaneCandidate{ID: "L2", Tier: TierMaybe, Files: []string{"b/b.go"}})
	q.Enqueue(LaneCandidate{ID: "IFFY", Tier: TierIffy, Files: []string{"core/c.go"}})

	got := q.ComposePrefixes()
	// L1, L1+L2 compose; IFFY is solo.
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

// TestPrefixQueue_ResolveCulprit names ResolveCulprit and pins positional NNFI
// ejection in a linear verify budget.
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

// TestPrefixQueueType_ZeroValue names the PrefixQueue and RiskTier types by their
// bare identifiers and pins the zero-value composer: an unstarted queue still
// composes an enqueued lane (window is unused by ComposePrefixes).
func TestPrefixQueueType_ZeroValue(t *testing.T) {
	var q PrefixQueue
	var tier RiskTier = TierMaybe
	q.Enqueue(LaneCandidate{ID: "x", Tier: tier, Files: []string{"x/x.go"}})
	if got := q.ComposePrefixes(); !reflect.DeepEqual(got, [][]string{{"x"}}) {
		t.Errorf("zero-value ComposePrefixes() = %v, want [[x]]", got)
	}
}

// TestLandingMode_Vocabulary names LandingMode, its consts, DefaultLandingMode
// and ParseLandingMode, pinning the closed vocabulary + fail-loud parse.
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
