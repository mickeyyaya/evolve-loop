package policy_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestDeepTier_IsTheCanonicalDeepNameBetweenBalancedAndTop(t *testing.T) {
	t.Parallel()
	if policy.TierRank(policy.DeepTier) != 3 || policy.TierName(3) != policy.DeepTier {
		t.Errorf("DeepTier %q has rank %d and rank 3 is named %q, want the rank-3 name", policy.DeepTier, policy.TierRank(policy.DeepTier), policy.TierName(3))
	}
	if !(policy.TierRank("balanced") < policy.TierRank(policy.DeepTier) && policy.TierRank(policy.DeepTier) < policy.TierRank("top")) {
		t.Errorf("DeepTier must rank above balanced and below top")
	}
}
