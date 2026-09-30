//go:build acs

package cycle975

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func TestC975_001_PositionalCulpritEjectsAndReforms(t *testing.T) {
	q := fleet.NewPrefixQueue()
	q.Enqueue(fleet.LaneCandidate{ID: "L1", Tier: fleet.TierMaybe, Files: []string{"go/internal/a/a.go"}})
	q.Enqueue(fleet.LaneCandidate{ID: "L2", Tier: fleet.TierMaybe, Files: []string{"go/internal/b/b.go"}})
	q.Enqueue(fleet.LaneCandidate{ID: "L3", Tier: fleet.TierMaybe, Files: []string{"go/internal/c/c.go"}})

	calls := 0
	verify := func(laneIDs []string) bool {
		calls++
		return !contains(laneIDs, "L2")
	}

	landed, ejected := q.ResolveCulprit(verify)

	if !contains(landed, "L1") || !contains(landed, "L3") {
		t.Errorf("expected L1 and L3 to land, got landed=%v", landed)
	}
	if contains(landed, "L2") {
		t.Errorf("poisoned lane L2 must not land, got landed=%v", landed)
	}
	if len(ejected) != 1 || ejected[0] != "L2" {
		t.Errorf("expected exactly L2 ejected, got ejected=%v", ejected)
	}
	if calls > 6 {
		t.Errorf("verify called %d times for 3 lanes — expected linear NNFI resolution (<=6), not a bisection sweep", calls)
	}
}

func TestC975_002_AIMDWindowAdaptsToPassRate(t *testing.T) {
	q := fleet.NewPrefixQueue()

	if got := q.Window(); got != 3 {
		t.Errorf("initial window = %d, want 3", got)
	}
	q.OnGreen()
	q.OnGreen()
	if got := q.Window(); got != 5 {
		t.Errorf("window after 2 greens = %d, want 5", got)
	}
	q.OnRed()
	if got := q.Window(); got != 2 {
		t.Errorf("window after red = %d, want 2 (halved from 5)", got)
	}
	q.OnRed()
	if got := q.Window(); got != 1 {
		t.Errorf("window after red = %d, want 1 (halved from 2)", got)
	}
	q.OnRed()
	if got := q.Window(); got != 1 {
		t.Errorf("window after red at floor = %d, want 1 (floor)", got)
	}
	q.OnGreen()
	if got := q.Window(); got != 2 {
		t.Errorf("window after green from floor = %d, want 2", got)
	}
}

func TestC975_003_IffyTierGetsSoloSlot(t *testing.T) {
	q := fleet.NewPrefixQueue()
	q.Enqueue(fleet.LaneCandidate{ID: "L1", Tier: fleet.TierMaybe, Files: []string{"go/internal/a/a.go"}})
	q.Enqueue(fleet.LaneCandidate{ID: "IFFY", Tier: fleet.TierIffy, Files: []string{"go/internal/core/core.go"}})
	q.Enqueue(fleet.LaneCandidate{ID: "L3", Tier: fleet.TierMaybe, Files: []string{"go/internal/c/c.go"}})

	for _, prefix := range q.ComposePrefixes() {
		if contains(prefix, "IFFY") && len(prefix) != 1 {
			t.Errorf("iffy lane must be solo, found it in multi-lane prefix %v", prefix)
		}
	}

	q2 := fleet.NewPrefixQueue()
	q2.Enqueue(fleet.LaneCandidate{ID: "X1", Tier: fleet.TierMaybe, Files: []string{"go/internal/shared/s.go"}})
	q2.Enqueue(fleet.LaneCandidate{ID: "X2", Tier: fleet.TierMaybe, Files: []string{"go/internal/shared/s.go"}})

	for _, prefix := range q2.ComposePrefixes() {
		if contains(prefix, "X1") && contains(prefix, "X2") {
			t.Errorf("overlap-zone lanes X1,X2 (shared file) must not share a prefix, got %v", prefix)
		}
	}
}

func TestC975_004_LandingModePolicyVocabulary(t *testing.T) {
	if got := fleet.DefaultLandingMode(); got != fleet.LandingPerLane {
		t.Errorf("default landing mode = %q, want %q", got, fleet.LandingPerLane)
	}
	if m, err := fleet.ParseLandingMode("per-lane"); err != nil || m != fleet.LandingPerLane {
		t.Errorf(`ParseLandingMode("per-lane") = (%q,%v), want (%q,nil)`, m, err, fleet.LandingPerLane)
	}
	if m, err := fleet.ParseLandingMode("prefix-queue"); err != nil || m != fleet.LandingPrefixQueue {
		t.Errorf(`ParseLandingMode("prefix-queue") = (%q,%v), want (%q,nil)`, m, err, fleet.LandingPrefixQueue)
	}
	if _, err := fleet.ParseLandingMode("bogus"); err == nil {
		t.Errorf(`ParseLandingMode("bogus") = nil error, want a validation error`)
	}
	if _, err := fleet.ParseLandingMode(strings.TrimSpace("  ")); err == nil {
		t.Errorf(`ParseLandingMode("") = nil error, want a validation error`)
	}
}
