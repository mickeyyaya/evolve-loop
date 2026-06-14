package profiles_test

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestAllProfileDefaultTiersResolveForSwappableDrivers(t *testing.T) {
	t.Parallel()

	drivers := swappableDriverManifests()
	manifests := loadSwappableManifests(t, drivers)
	canonical := canonicalTierSet()

	loader := profiles.NewFromDir(filepath.Join("..", "..", "..", ".evolve", "profiles"))
	names, err := loader.List()
	if err != nil {
		t.Fatalf("List real profiles: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("List real profiles returned no profiles")
	}

	for _, name := range names {
		profile, err := loader.Get(name)
		if err != nil {
			t.Fatalf("Get(%q): %v", name, err)
		}
		tier := profile.ModelTierDefault
		if !canonical[tier] {
			t.Errorf("%s: model_tier_default=%q, want one of %v", name, tier, modelcatalog.CanonicalTiers)
			continue
		}
		for _, driver := range drivers {
			if model := manifests[driver.manifest].ModelTierMap[tier]; model == "" {
				t.Errorf("%s: %s manifest %q has no model for default tier %q", name, driver.family, driver.manifest, tier)
			}
		}
	}
}

func TestSwappableDriverManifestsCoverCanonicalTiersOnly(t *testing.T) {
	t.Parallel()

	drivers := swappableDriverManifests()
	manifests := loadSwappableManifests(t, drivers)
	canonical := canonicalTierSet()

	for _, driver := range drivers {
		models := manifests[driver.manifest].ModelTierMap
		for _, tier := range modelcatalog.CanonicalTiers {
			if models[tier] == "" {
				t.Errorf("%s manifest %q missing canonical tier %q", driver.family, driver.manifest, tier)
			}
		}
		for tier := range models {
			if !canonical[tier] {
				t.Errorf("%s manifest %q has non-canonical tier key %q", driver.family, driver.manifest, tier)
			}
		}
	}
}

type swappableDriverManifest struct {
	family   string
	manifest string
}

func swappableDriverManifests() []swappableDriverManifest {
	return []swappableDriverManifest{
		{family: "codex", manifest: "codex"},
		{family: "codex", manifest: "codex-tmux"},
		{family: "agy", manifest: "agy"},
		{family: "agy", manifest: "agy-tmux"},
		{family: "ollama", manifest: "ollama-tmux"},
	}
}

func loadSwappableManifests(t *testing.T, drivers []swappableDriverManifest) map[string]bridge.Manifest {
	t.Helper()

	manifests := make(map[string]bridge.Manifest, len(drivers))
	for _, driver := range drivers {
		manifest, err := bridge.LoadManifest(driver.manifest)
		if err != nil {
			t.Fatalf("LoadManifest(%q): %v", driver.manifest, err)
		}
		if len(manifest.ModelTierMap) == 0 {
			t.Fatalf("LoadManifest(%q) returned an empty model tier map", driver.manifest)
		}
		manifests[driver.manifest] = manifest
	}
	return manifests
}

func canonicalTierSet() map[string]bool {
	tiers := make(map[string]bool, len(modelcatalog.CanonicalTiers))
	for _, tier := range modelcatalog.CanonicalTiers {
		tiers[tier] = true
	}
	return tiers
}

func TestAllProfilesSubstitutabilityAtParity(t *testing.T) {
	t.Parallel()

	drivers := swappableDriverManifests()
	manifests := loadSwappableManifests(t, drivers)

	loader := profiles.NewFromDir(filepath.Join("..", "..", "..", ".evolve", "profiles"))
	names, err := loader.List()
	if err != nil {
		t.Fatalf("List real profiles: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("List real profiles returned no profiles")
	}

	checkTier := func(profileName, field, tier string) {
		if tier == "" {
			return
		}
		for _, driver := range drivers {
			if manifests[driver.manifest].ModelTierMap[tier] == "" {
				t.Errorf("%s [field=%s driver=%s]: tier %q not in manifest", profileName, field, driver.manifest, tier)
			}
		}
	}

	for _, name := range names {
		profile, err := loader.Get(name)
		if err != nil {
			t.Fatalf("Get(%q): %v", name, err)
		}
		checkTier(name, "model_tier_default", profile.ModelTierDefault)
		for k, v := range profile.ModelTierOverrides {
			checkTier(name, "model_tier_overrides["+k+"]", v)
		}
		if env := profile.ModelTierEnvelope; env != nil {
			checkTier(name, "model_tier_envelope.min", env.Min)
			checkTier(name, "model_tier_envelope.default", env.Default)
			checkTier(name, "model_tier_envelope.max", env.Max)
		}
	}
}
