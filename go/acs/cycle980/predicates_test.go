//go:build acs

package cycle980

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
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

func shippedProfile(t *testing.T, name string) *profiles.Profile {
	t.Helper()
	loader := profiles.NewFromDir(filepath.Join(acsassert.RepoRoot(t), ".evolve", "profiles"))
	prof, err := loader.Get(name)
	if err != nil {
		t.Fatalf("load shipped profile %s: %v", name, err)
	}
	return &prof
}

func TestC980_001_InScopeProfilesCarryFastBalancedEnvelope(t *testing.T) {
	for _, name := range inScopeScanProfiles {
		prof := shippedProfile(t, name)
		env := prof.ModelTierEnvelope
		if env == nil {
			t.Errorf("RED %s: model_tier_envelope is nil — must be {min:fast, max:balanced}", name)
			continue
		}
		if env.Min != "fast" || env.Max != "balanced" {
			t.Errorf("%s: model_tier_envelope = {min:%q, max:%q}, want {min:fast, max:balanced}", name, env.Min, env.Max)
		}
	}
}

func TestC980_002_EnvelopeClampsDeepAndAdmitsFast(t *testing.T) {
	for _, name := range inScopeScanProfiles {
		prof := shippedProfile(t, name)
		if err := policy.ValidatePin(name, policy.Pin{Model: "deep"}, prof); err == nil {
			t.Errorf("RED %s: ValidatePin admitted a deep pin — the {fast,balanced} envelope must clamp it", name)
		}
		if err := policy.ValidatePin(name, policy.Pin{Model: "fast"}, prof); err != nil {
			t.Errorf("%s: ValidatePin rejected a fast pin (the envelope min); envelope must admit fast: %v", name, err)
		}
	}
}

func TestC980_003_ExcludedProfilesUntouched(t *testing.T) {
	for _, name := range excludedScanProfiles {
		prof := shippedProfile(t, name)
		if env := prof.ModelTierEnvelope; env != nil && env.Min == "fast" && env.Max == "balanced" {
			t.Errorf("%s: gained a {fast,balanced} envelope but is OUT of scope (owned by mechanical-scans-to-native)", name)
		}
		if err := policy.ValidatePin(name, policy.Pin{Model: "deep"}, prof); err != nil {
			t.Errorf("%s: a deep pin was rejected — this excluded profile's floor must be left untouched: %v", name, err)
		}
	}
}

// acs-predicate: config-check
func TestC980_004_ReportSizeBudgetWithinBand(t *testing.T) {
	pol, err := policy.Load(filepath.Join(acsassert.RepoRoot(t), ".evolve", "policy.json"))
	if err != nil {
		t.Fatalf("load shipped policy.json: %v", err)
	}
	got := pol.ReportBudgetConfig().HandoffTokens
	if got < 1000 || got > 2000 {
		t.Errorf("report-size handoff budget = %d tokens, want within the 1000–2000 band", got)
	}
}
