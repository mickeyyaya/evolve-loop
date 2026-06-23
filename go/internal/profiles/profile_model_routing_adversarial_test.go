package profiles_test

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolveloop/go/internal/profiles"
)

// TestAllProfilesModelTierDefaultIsNonEmpty guards against profiles with an
// empty model_tier_default. TestAllProfilesSubstitutabilityAtParity silently
// skips empty tiers (checkTier returns early), so a missing default would pass
// the parity check without detection.
func TestAllProfilesModelTierDefaultIsNonEmpty(t *testing.T) {
	t.Parallel()

	loader := profiles.NewFromDir(filepath.Join("..", "..", "..", ".evolve", "profiles"))
	names, err := loader.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("List returned no profiles")
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

// TestAllProfilesModelTierOverridesValuesAreCanonical checks that override
// values are explicitly in modelcatalog.CanonicalTiers, not just resolvable in
// the current driver manifests. Manifest-only resolution could silently accept a
// non-canonical tier that was added to a manifest without being added to
// CanonicalTiers.
func TestAllProfilesModelTierOverridesValuesAreCanonical(t *testing.T) {
	t.Parallel()

	canonical := canonicalTierSet()
	loader := profiles.NewFromDir(filepath.Join("..", "..", "..", ".evolve", "profiles"))
	names, err := loader.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

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

// TestEnvelopeTierHierarchyOrdering validates that when a profile declares a
// ModelTierEnvelope, the min/default/max fields are in non-decreasing order
// in the canonical tier hierarchy (fast < balanced < deep). Individual field
// resolution against driver manifests does not catch a semantically inverted
// envelope like {min: "deep", default: "balanced", max: "fast"}.
func TestEnvelopeTierHierarchyOrdering(t *testing.T) {
	t.Parallel()

	tierRank := make(map[string]int, len(modelcatalog.CanonicalTiers))
	for i, tier := range modelcatalog.CanonicalTiers {
		tierRank[tier] = i
	}

	loader := profiles.NewFromDir(filepath.Join("..", "..", "..", ".evolve", "profiles"))
	names, err := loader.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

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

// TestRestrictedClisProfilesTiersAreDriverAgnostic explicitly tests the policy
// invariant documented in model-routing-policy.md: profiles with intentional
// allowed_clis restrictions (builder, tdd-engineer, tester) constrain dispatch
// eligibility but must still express tier vocabulary that resolves in ALL
// swappable driver manifests. Dispatch restriction ≠ tier vocabulary restriction.
func TestRestrictedClisProfilesTiersAreDriverAgnostic(t *testing.T) {
	t.Parallel()

	restricted := []string{"builder", "tdd-engineer", "tester"}
	canonical := canonicalTierSet()
	drivers := swappableDriverManifests()
	manifests := loadSwappableManifests(t, drivers)
	loader := profiles.NewFromDir(filepath.Join("..", "..", "..", ".evolve", "profiles"))

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

// TestSubstitutabilityParityCoversMinimumProfiles guards against a reduced
// profiles directory silently weakening substitutability coverage. If fewer than
// 80 profiles are loaded, the parity test passes with less than full coverage.
func TestSubstitutabilityParityCoversMinimumProfiles(t *testing.T) {
	t.Parallel()

	const minExpectedProfiles = 80

	loader := profiles.NewFromDir(filepath.Join("..", "..", "..", ".evolve", "profiles"))
	names, err := loader.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) < minExpectedProfiles {
		t.Errorf("loaded %d profiles, want at least %d — substitutability parity coverage may be inadequate",
			len(names), minExpectedProfiles)
	}
}
