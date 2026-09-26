package profiles

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

func TestSpineProfilesAreDriverAgnostic(t *testing.T) {
	canonical := make(map[string]bool, len(modelcatalog.CanonicalTiers))
	for _, tier := range modelcatalog.CanonicalTiers {
		canonical[tier] = true
	}

	spine := []string{"scout", "triage", "tdd-engineer", "builder", "auditor", "router", "reflector"}
	loader, _ := RealTreeProfiles(t)

	for _, name := range spine {
		p, err := loader.Get(name)
		if err != nil {
			t.Fatalf("load profile %s: %v", name, err)
		}
		checkDriverAgnosticProfile(t, name, p, canonical)
	}
}

func TestSpineSubstitutabilityAtParity(t *testing.T) {
	catalog := modelcatalog.Catalog{
		CLIs: map[string]modelcatalog.CLIEntry{
			"codex": {TierModels: map[string]string{
				"fast":     "gpt-5.4-mini",
				"balanced": "gpt-5.4",
				"deep":     "gpt-5.5",
			}},
			"agy": {TierModels: map[string]string{
				"fast":     "gemini-flash-low",
				"balanced": "gemini-flash-high",
				"deep":     "claude-opus",
			}},
			"ollama": {TierModels: map[string]string{
				"fast":     "phi4",
				"balanced": "llama3.3",
				"deep":     "gemma4:31b-cloud",
			}},
		},
	}

	spine := []string{"scout", "triage", "tdd-engineer", "builder", "auditor", "router", "reflector"}
	altDrivers := []string{"codex", "agy", "ollama"}
	loader, _ := RealTreeProfiles(t)

	for _, name := range spine {
		p, err := loader.Get(name)
		if err != nil {
			t.Fatalf("load profile %s: %v", name, err)
		}
		for _, driver := range altDrivers {
			model, ok := catalog.Lookup(driver, p.ModelTierDefault)
			if !ok || model == "" {
				t.Errorf("%s: modelcatalog.Lookup(%q,%q) = (%q,%v), want non-empty model at equal capability tier",
					name, driver, p.ModelTierDefault, model, ok)
			}
		}
	}
}

func TestAllProfilesAreDriverAgnostic(t *testing.T) {
	canonical := make(map[string]bool, len(modelcatalog.CanonicalTiers))
	for _, tier := range modelcatalog.CanonicalTiers {
		canonical[tier] = true
	}

	loader, names := RealTreeProfiles(t)

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			p, err := loader.Get(name)
			if err != nil {
				t.Fatalf("load profile %s: %v", name, err)
			}
			checkDriverAgnosticProfile(t, name, p, canonical)
		})
	}
}

func checkDriverAgnosticProfile(t *testing.T, name string, p Profile, canonical map[string]bool) {
	t.Helper()
	if !canonical[p.ModelTierDefault] {
		t.Errorf("%s: model_tier_default=%q is not a canonical tier %v — a vendor model name is not driver-agnostic (misses modelcatalog.Lookup for non-Claude CLIs)",
			name, p.ModelTierDefault, modelcatalog.CanonicalTiers)
	}
	for key, tier := range p.ModelTierOverrides {
		if !canonical[tier] {
			t.Errorf("%s: model_tier_overrides[%q]=%q is not a canonical tier — hard-codes a vendor model", name, key, tier)
		}
	}
	if env := p.ModelTierEnvelope; env != nil {
		for label, tier := range map[string]string{"min": env.Min, "default": env.Default, "max": env.Max} {
			if tier != "" && !canonical[tier] {
				t.Errorf("%s: model_tier_envelope.%s=%q is not a canonical tier", name, label, tier)
			}
		}
	}
}
