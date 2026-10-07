package policy_test

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

var inScopeScanProfiles = []string{
	"authz-gap-scan",
	"cache-strategy-scan",
	"container-hardening-scan",
	"coverage-gate",
	"error-handling-scan",
	"query-performance-scan",
	"race-condition-scan",
	"resilience-gap-scan",
	"security-scan",
	"smell-scan",
	"test-amplification",
}

var excludedScanProfiles = []string{"secret-leak-scan", "flake-rerun-scan"}

func loadShippedScanProfile(t *testing.T, name string) *profiles.Profile {
	t.Helper()
	loader := profiles.NewFromDir(filepath.Join("..", "..", "..", ".evolve", "profiles"))
	prof, err := loader.Get(name)
	if err != nil {
		t.Fatalf("load shipped profile %s: %v", name, err)
	}
	return &prof
}

func TestScanProfiles_CarryABalancedOnlyEnvelope(t *testing.T) {
	for _, name := range inScopeScanProfiles {
		prof := loadShippedScanProfile(t, name)
		env := prof.ModelTierEnvelope
		if env == nil {
			t.Errorf("%s: model_tier_envelope is nil — want {min:balanced, max:balanced}", name)
			continue
		}
		if env.Min != "balanced" || env.Max != "balanced" {
			t.Errorf("%s: model_tier_envelope = {min:%q, max:%q}, want {min:balanced, max:balanced}: scans run on Flash High, never Flash Low", name, env.Min, env.Max)
		}
	}
}

func TestScanProfiles_EnvelopeClampsDeepAndFastPins(t *testing.T) {
	for _, name := range inScopeScanProfiles {
		prof := loadShippedScanProfile(t, name)
		for _, tier := range []string{"deep", "fast"} {
			if err := policy.ValidatePin(name, policy.Pin{Model: tier}, prof); err == nil {
				t.Errorf("%s: ValidatePin admitted a %s pin — the balanced-only envelope must clamp it", name, tier)
			}
		}
		if err := policy.ValidatePin(name, policy.Pin{Model: "balanced"}, prof); err != nil {
			t.Errorf("%s: ValidatePin rejected a balanced pin (the envelope itself): %v", name, err)
		}
	}
}

func TestScanProfiles_ExcludedUntouched(t *testing.T) {
	for _, name := range excludedScanProfiles {
		prof := loadShippedScanProfile(t, name)
		if env := prof.ModelTierEnvelope; env != nil && env.Max == "balanced" {
			t.Errorf("%s: gained a balanced-capped envelope but is out of scope (owned by mechanical-scans-to-native)", name)
		}
		if err := policy.ValidatePin(name, policy.Pin{Model: "deep"}, prof); err != nil {
			t.Errorf("%s: a deep pin was rejected — this excluded profile's floor must stay untouched: %v", name, err)
		}
	}
}

func TestScanEnvelope_ValidatePinStillClamps(t *testing.T) {
	prof := &profiles.Profile{ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "fast", Max: "balanced"}}
	if err := policy.ValidatePin("scan", policy.Pin{Model: "deep"}, prof); err == nil {
		t.Errorf("ValidatePin must reject a deep pin under a {fast,balanced} envelope; got nil (enforcement gutted)")
	}
	if err := policy.ValidatePin("scan", policy.Pin{Model: "fast"}, prof); err != nil {
		t.Errorf("ValidatePin must admit a fast pin under a {fast,balanced} envelope; got %v", err)
	}
}
