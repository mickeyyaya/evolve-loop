package convergence

import (
	"cmp"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

func topTier() string { return modelcatalog.CanonicalTiers[len(modelcatalog.CanonicalTiers)-1] }

func (h HeadroomTable) tierAbove(family, tier string) string {
	i := slices.Index(modelcatalog.CanonicalTiers, tier)
	if i < 0 {
		return ""
	}
	for _, above := range modelcatalog.CanonicalTiers[i+1:] {
		if _, ok := h[family][above]; ok {
			return above
		}
	}
	return ""
}

func (h HeadroomTable) raise(from TierEffort, tier string) (TierEffort, bool) {
	current, hasCurrent := h[from.Family][from.Tier]
	target, hasTarget := h[from.Family][tier]
	actual := ModelEffort{Model: cmp.Or(from.Model, current.Model), Effort: cmp.Or(from.Effort, current.Effort)}
	if !hasCurrent || !hasTarget || target == current || target == actual {
		return TierEffort{}, false
	}
	return TierEffort{Family: from.Family, Tier: tier, Model: target.Model, Effort: target.Effort}, true
}
