package fleet

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/prefixqueue"
)

func TestPrefixQueueAliases_AreThePrefixQueuePackage(t *testing.T) {
	t.Parallel()
	var q *PrefixQueue = NewPrefixQueue()
	var tier RiskTier = TierIffy
	q.Enqueue(LaneCandidate{ID: "x", Tier: tier, Files: []string{"x/x.go"}})
	var moved *prefixqueue.PrefixQueue = q
	if got := moved.ComposePrefixes(); !reflect.DeepEqual(got, [][]string{{"x"}}) {
		t.Errorf("ComposePrefixes() through the alias = %v, want [[x]]", got)
	}
	if TierRollup != prefixqueue.TierRollup || TierMaybe != prefixqueue.TierMaybe || TierIffy != prefixqueue.TierIffy {
		t.Error("the fleet tier constants differ from the prefixqueue constants")
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
