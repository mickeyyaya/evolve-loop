package bridge

import (
	"bytes"
	"strings"
	"testing"
)

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
	if got := stderr.String(); !strings.Contains(got, "deprecated") || !strings.Contains(got, "test-cli") {
		t.Errorf("expected stderr deprecation warning naming cli; got: %q", got)
	}
}

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
