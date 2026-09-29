package core

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/addedtests"
)

func TestTaggedTestArgs_RunAnAddedPackageUnderShipsBackstopBudget(t *testing.T) {
	got := taggedTestArgs("./acs/cycle1763", []string{"acs"})

	want := []string{"test", "-count=1", "-timeout", addedtests.PackageTimeout, "-tags", "acs", "./acs/cycle1763"}
	if !slices.Equal(got, want) {
		t.Errorf("args = %v, want %v: the floor runs the added package exactly as its twin, ship's added-test backstop, does, never under a stricter deadline (at 120s the floor killed cycle 1763's own -count=50 -race predicate and the lane edited the floor to survive)", got, want)
	}
}
