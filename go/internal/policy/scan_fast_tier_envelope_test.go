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
	"telemetry-coverage-check",
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

func TestScanProfiles_CarryFastTierEnvelope(t *testing.T) {
	for _, name := range inScopeScanProfiles {
		prof := loadShippedScanProfile(t, name)
		env := prof.ModelTierEnvelope
		if env == nil {
			t.Errorf("%s: model_tier_envelope is nil — want {min:fast, max:balanced}", name)
			continue
		}
		if env.Min != "fast" || env.Max != "balanced" {
			t.Errorf("%s: model_tier_envelope = {min:%q, max:%q}, want {min:fast, max:balanced}", name, env.Min, env.Max)
		}
	}
}

func TestScanProfiles_EnvelopeClampsDeepPin(t *testing.T) {
	for _, name := range inScopeScanProfiles {
		prof := loadShippedScanProfile(t, name)
		if err := policy.ValidatePin(name, policy.Pin{Model: "deep"}, prof); err == nil {
			t.Errorf("%s: ValidatePin admitted a deep pin — the {fast,balanced} envelope must clamp it", name)
		}
		if err := policy.ValidatePin(name, policy.Pin{Model: "fast"}, prof); err != nil {
			t.Errorf("%s: ValidatePin rejected a fast pin (the envelope min): %v", name, err)
		}
	}
}

func TestScanProfiles_ExcludedUntouched(t *testing.T) {
	for _, name := range excludedScanProfiles {
		prof := loadShippedScanProfile(t, name)
		if env := prof.ModelTierEnvelope; env != nil && env.Min == "fast" && env.Max == "balanced" {
			t.Errorf("%s: gained a {fast,balanced} envelope but is out of scope (owned by mechanical-scans-to-native)", name)
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
