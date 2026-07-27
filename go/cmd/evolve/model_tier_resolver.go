package main

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// resolveModelTier is the (cli,tier)→(model,ok) probe that
// router.ClampPlanModelRouting consults before accepting an advisor's model
// routing. The clamp treats ok=false as a breach and clears the proposed
// {cli,tier} back to the profile default, so this predicate must answer
// exactly "would a dispatch of this pairing resolve a model?" — nothing
// looser, nothing stricter.
//
// It resolves through bridge.LoadManifest, the SAME path a real launch takes:
// LoadManifest already merges the live model-catalog overlay (source=="live"
// entries only) over the manifest's model_tier_map, so the answer tracks
// catalog freshness without the gate having to know the catalog exists.
//
// Deliberately NOT modelcatalog.Catalog.Lookup: that consults only the cached
// catalog and returns ok=false for every CLI whose entry is source:"detect" —
// codex today, frozen there since its /model picker capture began failing.
// Gating on it would clear codex out of every advisor plan even though
// dispatch resolves codex tiers fine from the manifest baseline. Lookup is
// also ungated on provenance, so it would have reported the identity-fallback
// garbage (claude top→"top") as perfectly resolvable.
//
// A plain function, not a make* factory: it captures no composition-root
// config, unlike makeCatalogRefresher/makeDirectivesProvider whose prefix
// signals exactly that. Passed by value at the composition root so core and
// router stay leaves and never import bridge or modelcatalog.
func resolveModelTier(cli, tier string) (string, bool) {
	m, err := bridge.LoadManifest(policy.BaseCLI(cli) + "-tmux")
	if err != nil {
		return "", false
	}
	model := m.ModelTierMap[tier]
	return model, model != ""
}
