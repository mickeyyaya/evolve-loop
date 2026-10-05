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

func TestUnitTestArgs_RunAChangedPackageUnderTheSharedBudget(t *testing.T) {
	got := unitTestArgs("./cmd/evolve")

	want := []string{"test", "-count=1", "-timeout", addedtests.PackageTimeout, "./cmd/evolve"}
	if !slices.Equal(got, want) {
		t.Errorf("args = %v, want %v: the floor's plain unit check must run a changed package under the one go-test budget; at 120s a green ./cmd/evolve (105-127s on the loop host) timed out on every correction round and failed cycles 1787, 1791, 1792 and 1798", got, want)
	}
}

func TestCoverTestArgs_RunTheCoveragePassUnderTheSharedBudget(t *testing.T) {
	got := coverTestArgs("/tmp/cover.out", []string{"./cmd/evolve", "./internal/core"})

	want := []string{"test", "-count=1", "-timeout", addedtests.PackageTimeout, "-coverprofile", "/tmp/cover.out", "./cmd/evolve", "./internal/core"}
	if !slices.Equal(got, want) {
		t.Errorf("args = %v, want %v: the floor's coverage pass runs the same packages as its unit check, so it must use the same go-test budget, not a 300s literal of its own", got, want)
	}
}
