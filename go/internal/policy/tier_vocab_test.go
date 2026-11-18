package policy_test

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestTierNames_EqualTheCanonicalCatalogTiers(t *testing.T) {
	if got := policy.TierNames(); !slices.Equal(got, modelcatalog.CanonicalTiers) {
		t.Fatalf("policy.TierNames() = %v, modelcatalog.CanonicalTiers = %v: one tier vocabulary, two copies drifted", got, modelcatalog.CanonicalTiers)
	}
}

func TestTierRankAndName_FollowTheCanonicalCatalogOrder(t *testing.T) {
	for i, name := range modelcatalog.CanonicalTiers {
		if rank := policy.TierRank(name); rank != i+1 {
			t.Errorf("policy.TierRank(%q) = %d, want %d", name, rank, i+1)
		}
		if got := policy.TierName(i + 1); got != name {
			t.Errorf("policy.TierName(%d) = %q, want %q", i+1, got, name)
		}
	}
}
