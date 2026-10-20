package cliroute

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

func TestShareBucket_TheBucketsOfTheSpecExample(t *testing.T) {
	t.Parallel()
	for cycle, want := range map[int]int{1741: 1, 1742: 52} {
		if got := shareBucket(Request{Cycle: cycle, Agent: "builder", Phase: "build"}); got != want {
			t.Errorf("bucket of the builder of cycle %d = %d, want %d (docs/architecture/deep-tier-share.md §2)", cycle, got, want)
		}
	}
}

func TestDispatchTier_APlanWithoutTiersUsesItsModel(t *testing.T) {
	t.Parallel()
	if got := dispatchTier(llmroute.Plan{Model: "deep"}); got != "deep" {
		t.Fatalf("dispatchTier = %q, want the model deep", got)
	}
}

func TestWindowLabel_AFamilyWindowIsNamedByItsFamily(t *testing.T) {
	t.Parallel()
	if got := windowLabel(quotastate.UsageWindow{Family: "agy"}); got != "agy" {
		t.Fatalf("windowLabel = %q, want agy", got)
	}
}

func TestResolverNow_WithoutAHostClockTheWallClockIsUsed(t *testing.T) {
	t.Parallel()
	before := time.Now()
	if got := (resolver{}).now(); got.Before(before) || got.After(time.Now()) {
		t.Fatalf("now = %v, want the wall clock", got)
	}
}
