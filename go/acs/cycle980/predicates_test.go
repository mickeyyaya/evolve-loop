//go:build acs

// Package cycle980 materializes the cycle-980 acceptance criteria for this
// fleet lane's single scoped id `scan-phase-fast-tier-envelopes`.
//
// Goal (scout-report.md). Give judgment-light scan-phase profiles an explicit
// `model_tier_envelope` of {min:"fast", max:"balanced"} — overriding the
// universal {balanced,deep} floor — so their handoff-summary work can run at the
// fast tier while capping escalation at balanced. Twelve scan/checklist profiles
// are in scope; `secret-leak-scan.json` and `flake-rerun-scan.json` are OUT of
// scope (owned by `mechanical-scans-to-native`, which converts them to native
// Go) and MUST be left untouched.
//
// PREDICATE STYLE (cycle-85 rule): go/internal/{policy,profiles} are importable
// from go/acs, so the load-bearing predicates EXERCISE the live resolver —
// `policy.ValidatePin` — against the SHIPPED profiles (loaded from the on-disk
// worktree via acsassert.RepoRoot). C980_001 asserts the envelope *shape* and
// C980_002 asserts the envelope actually *clamps* by feeding ValidatePin a
// "deep" pin and requiring it to error. A magic-string source/config edit that
// does not produce a {fast,balanced} envelope the resolver honours cannot
// satisfy them.
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	NEGATIVE → C980_002 "reject deep" is the anti-no-op signal: a deep pin
//	           (rank 3) must be REJECTED for each of the 12 profiles once the
//	           {fast,balanced} envelope exists. RED today (nil envelope ⇒
//	           ValidatePin returns nil ⇒ no rejection).
//	EDGE/SCOPE→ C980_003 pins the OUT-OF-SCOPE boundary: the two excluded
//	           profiles must NOT gain a fast envelope and must STILL accept a
//	           deep pin. Green today; fails only if the Builder over-reaches.
//	SEMANTIC → C980_001 (shape: Min=="fast"/Max=="balanced"), C980_002
//	           (enforcement: clamp deep, admit fast), C980_004 (report-size
//	           budget stays in the 1–2K band) are distinct outcomes.
//
// RED before Builder: C980_001 fails (ModelTierEnvelope is nil on all 12) and
// C980_002 fails (ValidatePin admits a deep pin because there is no envelope to
// clamp against). C980_003 and C980_004 are regression guards — green today and
// MUST stay green after the change.
//
// AC map (1:1 with test-report.md AC-Materialization table):
//
//	AC1 12 profiles carry {min:fast,max:balanced}  → C980_001 + C980_002 (predicate)
//	AC2 excluded profiles left untouched            → C980_003 (predicate)
//	AC4 report-size budget resolves within 1–2K     → C980_004 (predicate, config-check)
package cycle980

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// inScopeScanProfiles are the 12 judgment-light scan/checklist profiles that
// must gain the {fast,balanced} envelope this cycle (scout-report.md line 13-14).
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

// excludedScanProfiles are owned by `mechanical-scans-to-native`; this task must
// not touch them (scout-report.md line 14, Decision Trace excluded_profiles).
var excludedScanProfiles = []string{"secret-leak-scan", "flake-rerun-scan"}

// shippedProfile loads a profile from the on-disk worktree .evolve/profiles dir
// (the same tree the Builder edits), so predicates pin the real shipped contract
// rather than a fixture.
func shippedProfile(t *testing.T, name string) *profiles.Profile {
	t.Helper()
	loader := profiles.NewFromDir(filepath.Join(acsassert.RepoRoot(t), ".evolve", "profiles"))
	prof, err := loader.Get(name)
	if err != nil {
		t.Fatalf("load shipped profile %s: %v", name, err)
	}
	return &prof
}

// TestC980_001_InScopeProfilesCarryFastBalancedEnvelope — AC1 (shape): each of
// the 12 in-scope scan profiles must declare model_tier_envelope with
// Min=="fast" and Max=="balanced". RED today: every one is nil.
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

// TestC980_002_EnvelopeClampsDeepAndAdmitsFast — AC1 (enforcement, exercises the
// SUT): with the {fast,balanced} envelope in place, the live policy.ValidatePin
// resolver must REJECT a deep pin (rank 3 above balanced) and ADMIT a fast pin
// (rank 1, the min). The deep-rejection is the load-bearing NEGATIVE: RED today
// because a nil envelope makes ValidatePin admit everything, so a no-op that
// leaves the envelope unset cannot pass.
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

// TestC980_003_ExcludedProfilesUntouched — AC2 (scope boundary): the two
// mechanical-scans-to-native profiles must NOT gain a {fast,balanced} envelope
// and must STILL admit a deep pin (their {balanced,deep} floor is unchanged).
// Green today; fails only if the Builder over-reaches into out-of-scope files.
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

// TestC980_004_ReportSizeBudgetWithinBand — AC4 (config-check): the report-size
// budget that governs these scan phases' handoff summaries must resolve inside
// the 1–2K token band the token-optimization research prescribes
// (knowledge-base/research/token-optimization-2026/README.md:26). Exercises the
// live policy.ReportBudgetConfig() against the shipped policy.json. Green today
// (default 2000, in band) and must stay in band — a guard against a config edit
// that drives the budget out of the research-backed range.
//
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
