package bridge

import (
	"bytes"
	"strings"
	"testing"
)

// manifest_v1_compat_test.go — pins the cycle-124-followup v1→v2 schema
// migration for the per-CLI parameter mapping table. v1 manifests use the
// Anthropic-leaked vocabulary `tier_aliases: {haiku|sonnet|opus → native}`;
// v2 manifests use the provider-neutral vocabulary `model_tier_map:
// {fast|balanced|deep → native}` matching what profiles already declare.
//
// The contract: a v1-shape manifest installed in EVOLVE_BRIDGE_MANIFEST_DIR
// (or hand-rolled by an operator) continues to work for one release after
// the migration. parseManifest detects the legacy key shape, translates
// haiku→fast / sonnet→balanced / opus→deep on read, populates the new
// ModelTierMap field, and emits ONE stderr deprecation line per manifest.
// After the deprecation window the v1 path is removed; this test fails
// loudly when the planned removal happens.

// TestManifestV1Compat_TranslatesTierAliasesKeys pins the read-side
// translation. A v1 manifest JSON with `tier_aliases` keys MUST realize
// to a Manifest whose ModelTierMap has the canonical abstract keys, with
// the same native model values.
func TestManifestV1Compat_TranslatesTierAliasesKeys(t *testing.T) {
	v1JSON := []byte(`{
		"cli": "test-cli",
		"binary": "test-bin",
		"tier_aliases": {
			"haiku":  "tiny-model",
			"sonnet": "mid-model",
			"opus":   "big-model"
		}
	}`)
	stderr := &bytes.Buffer{}
	m, err := parseManifestWithStderr("test-cli", v1JSON, stderr)
	if err != nil {
		t.Fatalf("parseManifest: %v", err)
	}
	// New abstract keys present with their original native values.
	want := map[string]string{
		"fast":     "tiny-model",
		"balanced": "mid-model",
		"deep":     "big-model",
	}
	if len(m.ModelTierMap) != len(want) {
		t.Fatalf("ModelTierMap=%v, want %v", m.ModelTierMap, want)
	}
	for k, v := range want {
		if got := m.ModelTierMap[k]; got != v {
			t.Errorf("ModelTierMap[%q]=%q, want %q", k, got, v)
		}
	}
	// MUST emit a deprecation warning on stderr naming the cli.
	if got := stderr.String(); !strings.Contains(got, "deprecated") || !strings.Contains(got, "test-cli") {
		t.Errorf("expected stderr deprecation warning naming cli; got: %q", got)
	}
}

// TestManifestV1Compat_PartialKeysTranslate covers a v1 manifest that
// declares only a subset of haiku/sonnet/opus (e.g., an operator override
// pinning just `opus`). The partial set translates verbatim; missing
// keys stay missing.
func TestManifestV1Compat_PartialKeysTranslate(t *testing.T) {
	v1JSON := []byte(`{
		"cli": "test-cli",
		"binary": "test-bin",
		"tier_aliases": {"opus": "only-the-big-one"}
	}`)
	m, err := parseManifestWithStderr("test-cli", v1JSON, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseManifest: %v", err)
	}
	if got, want := m.ModelTierMap["deep"], "only-the-big-one"; got != want {
		t.Errorf("ModelTierMap[deep]=%q, want %q", got, want)
	}
	if _, ok := m.ModelTierMap["fast"]; ok {
		t.Errorf("v1 partial translation must not invent missing keys; ModelTierMap[fast] present")
	}
	if _, ok := m.ModelTierMap["balanced"]; ok {
		t.Errorf("v1 partial translation must not invent missing keys; ModelTierMap[balanced] present")
	}
}

// TestManifestV2_LoadsModelTierMapDirectly covers the canonical v2 path:
// a manifest declaring `model_tier_map` is loaded verbatim, no translation,
// no deprecation warning.
func TestManifestV2_LoadsModelTierMapDirectly(t *testing.T) {
	v2JSON := []byte(`{
		"cli": "v2-cli",
		"binary": "v2-bin",
		"model_tier_map": {
			"fast":     "tiny-model",
			"balanced": "mid-model",
			"deep":     "big-model"
		}
	}`)
	stderr := &bytes.Buffer{}
	m, err := parseManifestWithStderr("v2-cli", v2JSON, stderr)
	if err != nil {
		t.Fatalf("parseManifest: %v", err)
	}
	want := map[string]string{"fast": "tiny-model", "balanced": "mid-model", "deep": "big-model"}
	if len(m.ModelTierMap) != len(want) {
		t.Fatalf("ModelTierMap=%v, want %v", m.ModelTierMap, want)
	}
	for k, v := range want {
		if got := m.ModelTierMap[k]; got != v {
			t.Errorf("ModelTierMap[%q]=%q, want %q", k, got, v)
		}
	}
	if stderr.Len() != 0 {
		t.Errorf("v2 manifest must not emit deprecation warning; stderr=%q", stderr.String())
	}
}

// TestManifestV1Compat_NonStandardKeysPassThrough pins that v1 keys
// outside the Anthropic triple (haiku/sonnet/opus) pass through to the
// new ModelTierMap verbatim. An operator override might have declared
// {"large": "..."} as a custom tier; the migration must not drop it.
func TestManifestV1Compat_NonStandardKeysPassThrough(t *testing.T) {
	v1JSON := []byte(`{
		"cli": "v1-custom",
		"binary": "x",
		"tier_aliases": {
			"haiku":  "tiny",
			"large":  "custom-large",
			"opus":   "big"
		}
	}`)
	m, err := parseManifestWithStderr("v1-custom", v1JSON, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseManifest: %v", err)
	}
	if got, want := m.ModelTierMap["fast"], "tiny"; got != want {
		t.Errorf("ModelTierMap[fast]=%q, want %q (haiku → fast)", got, want)
	}
	if got, want := m.ModelTierMap["deep"], "big"; got != want {
		t.Errorf("ModelTierMap[deep]=%q, want %q (opus → deep)", got, want)
	}
	if got, want := m.ModelTierMap["large"], "custom-large"; got != want {
		t.Errorf("ModelTierMap[large]=%q, want %q (custom key passes through)", got, want)
	}
}
