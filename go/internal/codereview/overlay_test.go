package codereview

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestTheCompiledOverlayRuleReachesThisPhase(t *testing.T) {
	got := policy.Policy{}.ResolveOverlays(policy.OverlayDispatch{
		Phase: PhaseName, Tier: "balanced",
		Signals: map[string]string{config.SignalDeliverableKind: config.DeliverableKindCode},
	})
	if !slices.Contains(got, "architecture-review") || !slices.Contains(got, "quality-index") || !slices.Contains(got, policy.SelfReviewSkill) {
		t.Errorf("a %s dispatch resolves %v, want the review skills: the overlay rule's phase literal must be codereview.PhaseName", PhaseName, got)
	}
}
