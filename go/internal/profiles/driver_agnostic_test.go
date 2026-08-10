package profiles

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

// realProfilesDir now lives in tracked_realtree_test.go beside the tracked-set
// funnel every real-tree test in this package goes through.

// TestSpineProfilesAreDriverAgnostic enforces the driver-agnostic invariant from
// docs/architecture/model-routing-policy.md: a spine phase profile MUST express
// every model-tier selection (default, overrides, envelope bounds) as an abstract
// capability tier from modelcatalog.CanonicalTiers ("fast"/"balanced"/"deep") —
// never a vendor model name (haiku/sonnet/opus).
//
// Why this matters: resolvellm passes model_tier_default through verbatim, and
// modelcatalog.Lookup(cli, tier) is keyed on the canonical tiers. A vendor name
// like "sonnet" therefore misses for every non-Claude driver
// (Lookup("codex","sonnet") == "", !ok), silently hard-coding Claude into the
// phase. Keeping the spine on canonical tiers is what makes Claude replaceable
// by codex/agy/ollama at equal capability — the per-driver concrete model is
// owned by modelcatalog (the single source of truth), not the profile.
//
// Scope note: this currently guards the 7 spine phases. The ~48 domain/optional
// profiles are migrating to canonical tiers via the loop; expand `spine` to the
// full fleet once that migration completes (tracked in model-routing-policy.md
// § Migration Status).
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

	// RealTreeProfiles binds only git-tracked profiles (untracked = runtime
	// mints, the cd49274beab2 false-RED class) and List() already excludes
	// non-profile JSON like tool-policy.json.
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
