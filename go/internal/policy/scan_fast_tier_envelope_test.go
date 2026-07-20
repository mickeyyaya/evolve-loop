package policy_test

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// scan_fast_tier_envelope_test.go — durable regression contract for cycle-980
// Task `scan-phase-fast-tier-envelopes` (inbox weight 0.94). The 12 in-scope
// judgment-light scan profiles must carry an explicit model_tier_envelope of
// {min:"fast", max:"balanced"} so their handoff work can run at the fast tier
// while capping escalation at balanced (overriding the universal {balanced,deep}
// floor). `secret-leak-scan`/`flake-rerun-scan` are OUT of scope (owned by
// mechanical-scans-to-native) and must be left on their existing floor.
//
// This is the PERMANENT sibling of the cycle-scoped ACS predicates in
// go/acs/cycle980 (AC3: "New Go regression test in internal/policy"). It reads
// the SHIPPED profiles via the same locator the memo/driver-agnostic tests use
// (filepath.Join("..","..","..",".evolve","profiles")) and exercises the live
// policy.ValidatePin resolver, so it pins the on-disk contract beyond this cycle.
//
// RED today: TestScanProfiles_CarryFastTierEnvelope and
// TestScanProfiles_EnvelopeClampsDeepPin fail because every in-scope profile's
// ModelTierEnvelope is nil. GREEN once the 12 profiles carry {fast,balanced}.

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

// TestScanProfiles_CarryFastTierEnvelope — AC1 (shape): each in-scope scan
// profile declares model_tier_envelope = {min:"fast", max:"balanced"}.
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

// TestScanProfiles_EnvelopeClampsDeepPin — AC1 (enforcement, exercises the live
// resolver): the {fast,balanced} envelope must make ValidatePin REJECT a deep
// pin and ADMIT a fast pin for every in-scope profile. The deep-rejection is the
// anti-no-op signal — a nil envelope (today) admits everything and red-fails it.
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

// TestScanProfiles_ExcludedUntouched — AC2 (scope boundary): the two
// mechanical-scans-to-native profiles must not gain a {fast,balanced} envelope
// and must still admit a deep pin. Green today; guards against over-reach.
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

// TestScanEnvelope_ValidatePinStillClamps — AC3 (enforcement-not-gutted): a
// fabricated {fast,balanced} envelope must still reject a deep pin through the
// real resolver. Green before and after the change; it fails only if someone
// "greens" the suite by weakening ValidatePin instead of adding the envelope.
func TestScanEnvelope_ValidatePinStillClamps(t *testing.T) {
	prof := &profiles.Profile{ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "fast", Max: "balanced"}}
	if err := policy.ValidatePin("scan", policy.Pin{Model: "deep"}, prof); err == nil {
		t.Errorf("ValidatePin must reject a deep pin under a {fast,balanced} envelope; got nil (enforcement gutted)")
	}
	if err := policy.ValidatePin("scan", policy.Pin{Model: "fast"}, prof); err != nil {
		t.Errorf("ValidatePin must admit a fast pin under a {fast,balanced} envelope; got %v", err)
	}
}
