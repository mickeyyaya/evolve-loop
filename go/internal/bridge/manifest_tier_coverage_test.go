package bridge

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

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
