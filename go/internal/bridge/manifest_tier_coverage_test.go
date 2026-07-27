package bridge

// manifest_tier_coverage_test.go — the SOURCE-side half of the tier-vocabulary
// invariant. realizer_tier_sentinel_test.go pins the SINK (a vocabulary token
// must never be emitted as a model); this pins the source (every tmux manifest
// must declare a real model for every canonical tier, so the sink guard never
// has to fire in production).
//
// Why both halves are needed: the sentinel alone would silently drop the model
// param for an undeclared tier, launching the CLI on whatever its own default
// happens to be — safe, but an invisible capability change for the 60+ profiles
// that carry no model_tier_envelope and can therefore be routed to "top"
// (router/model_routing_clamp.go universalTierFloor Max:"top").
//
// Glob-driven over ManifestNames() so a future *-tmux CLI cannot reintroduce
// the hole — no hardcoded CLI list.

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

// TestModelTierMap_EveryTmuxManifestCoversEveryCanonicalTier pins that each
// embedded tmux manifest maps every canonical tier to a concrete model id.
// RED before claude-tmux.json declares "top": setup.tierModelsFor's identity
// fallback then mints the literal string "top" as claude's top-tier model id.
func TestModelTierMap_EveryTmuxManifestCoversEveryCanonicalTier(t *testing.T) {
	injectCatalogDir(t, t.TempDir())

	for _, name := range ManifestNames() {
		m, err := LoadManifest(name)
		if err != nil {
			t.Fatalf("LoadManifest(%s): %v", name, err)
		}
		if !m.IsTmux() {
			continue
		}
		for _, tier := range modelcatalog.CanonicalTiers {
			t.Run(name+"/"+tier, func(t *testing.T) {
				model := m.ModelTierMap[tier]
				if model == "" {
					t.Errorf("%s model_tier_map has no %q entry — setup.tierModelsFor identity-falls-back to the tier NAME as the model id, "+
						"and Catalog.Lookup (ungated) then reports that garbage value as resolvable to the routing clamp",
						name, tier)
					return
				}
				if isUnresolvedModelToken(model) {
					t.Errorf("%s model_tier_map[%q] = %q — a manifest must map a tier to a concrete MODEL, never to vocabulary", name, tier, model)
				}
			})
		}
	}
}
