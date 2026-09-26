package profiles_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// The parity test skips empty tiers, so an empty default needs its own guard.
func TestAllProfilesModelTierDefaultIsNonEmpty(t *testing.T) {
	t.Parallel()

	loader, names := profiles.RealTreeProfiles(t)
	if len(names) == 0 {
		t.Fatal("RealTreeProfiles returned no profiles")
	}

	for _, name := range names {
		profile, err := loader.Get(name)
		if err != nil {
			t.Fatalf("Get(%q): %v", name, err)
		}
		if profile.ModelTierDefault == "" {
			t.Errorf("%s: model_tier_default is empty — every profile must declare a non-empty capability tier", name)
		}
	}
}

// Manifest resolution alone would accept a tier added to a manifest but not to CanonicalTiers.
func TestAllProfilesModelTierOverridesValuesAreCanonical(t *testing.T) {
	t.Parallel()

	canonical := canonicalTierSet()
	loader, names := profiles.RealTreeProfiles(t)

	for _, name := range names {
		profile, err := loader.Get(name)
		if err != nil {
			t.Fatalf("Get(%q): %v", name, err)
		}
		for k, v := range profile.ModelTierOverrides {
			if !canonical[v] {
				t.Errorf("%s: model_tier_overrides[%q]=%q is not a canonical tier (want one of %v)",
					name, k, v, modelcatalog.CanonicalTiers)
			}
		}
	}
}

func TestEnvelopeTierHierarchyOrdering(t *testing.T) {
	t.Parallel()

	tierRank := make(map[string]int, len(modelcatalog.CanonicalTiers))
	for i, tier := range modelcatalog.CanonicalTiers {
		tierRank[tier] = i
	}

	loader, names := profiles.RealTreeProfiles(t)

	for _, name := range names {
		profile, err := loader.Get(name)
		if err != nil {
			t.Fatalf("Get(%q): %v", name, err)
		}
		env := profile.ModelTierEnvelope
		if env == nil {
			continue
		}
		if env.Min != "" && env.Default != "" && tierRank[env.Min] > tierRank[env.Default] {
			t.Errorf("%s: envelope min=%q (rank %d) > default=%q (rank %d); min must be ≤ default",
				name, env.Min, tierRank[env.Min], env.Default, tierRank[env.Default])
		}
		if env.Default != "" && env.Max != "" && tierRank[env.Default] > tierRank[env.Max] {
			t.Errorf("%s: envelope default=%q (rank %d) > max=%q (rank %d); default must be ≤ max",
				name, env.Default, tierRank[env.Default], env.Max, tierRank[env.Max])
		}
		if env.Min != "" && env.Max != "" && tierRank[env.Min] > tierRank[env.Max] {
			t.Errorf("%s: envelope min=%q (rank %d) > max=%q (rank %d); min must be ≤ max",
				name, env.Min, tierRank[env.Min], env.Max, tierRank[env.Max])
		}
	}
}

// allowed_clis restricts dispatch, not tier vocabulary: these profiles' tiers
// must still resolve on every swappable driver.
func TestRestrictedClisProfilesTiersAreDriverAgnostic(t *testing.T) {
	t.Parallel()

	restricted := []string{"builder", "tdd-engineer", "tester"}
	canonical := canonicalTierSet()
	drivers := swappableDriverManifests()
	manifests := loadSwappableManifests(t, drivers)
	loader, _ := profiles.RealTreeProfiles(t)

	for _, name := range restricted {
		profile, err := loader.Get(name)
		if err != nil {
			t.Fatalf("Get(%q): %v — restricted profile must exist in the profiles directory", name, err)
		}
		tier := profile.ModelTierDefault
		if !canonical[tier] {
			t.Errorf("%s: model_tier_default=%q is not a canonical tier; allowed_clis restriction must not affect tier vocabulary",
				name, tier)
			continue
		}
		for _, driver := range drivers {
			if model := manifests[driver.manifest].ModelTierMap[tier]; model == "" {
				t.Errorf("%s: default tier %q not in %s manifest %q — tier vocabulary must resolve in all swappable drivers even for allowed_clis-restricted profiles",
					name, tier, driver.family, driver.manifest)
			}
		}
	}
}

func TestSubstitutabilityParityCoversMinimumProfiles(t *testing.T) {
	t.Parallel()

	const minExpectedProfiles = 80

	_, names := profiles.RealTreeProfiles(t)
	if len(names) < minExpectedProfiles {
		t.Errorf("loaded %d profiles, want at least %d — substitutability parity coverage may be inadequate",
			len(names), minExpectedProfiles)
	}
}

func TestModelTierOverridesWithinEnvelope(t *testing.T) {
	t.Parallel()

	tierRank := make(map[string]int, len(modelcatalog.CanonicalTiers))
	for i, tier := range modelcatalog.CanonicalTiers {
		tierRank[tier] = i
	}

	loader, names := profiles.RealTreeProfiles(t)
	if len(names) == 0 {
		t.Fatal("RealTreeProfiles returned no profiles")
	}

	for _, name := range names {
		profile, err := loader.Get(name)
		if err != nil {
			t.Fatalf("Get(%q): %v", name, err)
		}
		env := profile.ModelTierEnvelope
		if env == nil || env.Min == "" || env.Max == "" {
			continue
		}
		minR, okMin := tierRank[env.Min]
		maxR, okMax := tierRank[env.Max]
		if !okMin || !okMax {
			// Non-canonical bounds are TestAllProfilesAreDriverAgnostic's concern.
			continue
		}
		for key, tier := range profile.ModelTierOverrides {
			r, ok := tierRank[tier]
			if !ok {
				// Non-canonical values are TestAllProfilesModelTierOverridesValuesAreCanonical's concern.
				continue
			}
			if r < minR || r > maxR {
				t.Errorf("%s: model_tier_overrides[%q]=%q ranks outside its own envelope [min=%q, max=%q] — raise the override to the floor, or widen the envelope with a justifying comment",
					name, key, tier, env.Min, env.Max)
			}
		}
	}
}
