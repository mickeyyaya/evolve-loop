//go:build acs

// Package cycle517 materialises the cycle-517 acceptance criteria.
//
// TRIAGE COMMITTED EXACTLY ONE ## top_n TASK this cycle (triage-decision.json /
// triage-report.md — this is a fleet lane; the scout-report.md's two
// wave/boot-recovery findings were both DEFERRED — "not in the assigned fleet
// scope for this cycle's concurrent execution lane" — so, per R9.3, no
// predicates are authored for them here):
//
//	advisor-tier-vocab-add-top (carryover, priority=H, evidence=
//	go/internal/core/phase_advisor_tier_test.go:2) — "Wire the 'top' model tier
//	through advisor + policy rank". Investigation this cycle found the ADVISOR
//	half already landed in cycle 516 (sanitizeAdvisorTier accepts "top";
//	policy.TierRank classifies "top" as rank 4 — both pre-existing GREEN,
//	verified below as regression pins). The REMAINING gap is entirely within
//	go/internal/setup: package setup's own CONSUMERS of policy.TierRank's rank
//	4 were never updated —
//	  - tierFromRank (recommend.go) only maps ranks 1-3 back to a tier string,
//	    so canonTier("top") == "" (the setup/recommend flow cannot round-trip
//	    the literal string "top" at all).
//	  - biasTier's "up" strategy hard-caps at `if r < 3 { r++ }`, so it can
//	    never climb to "top" even when the envelope allows it.
//	  - abstractTiers (setup.go) is still the pre-4-tier {fast,balanced,deep}
//	    literal, so tierModelsFor never surfaces a "top" key at all.
//	The builtin "max-quality" preset (tier_bias="max") is broken end-to-end by
//	the first gap: biasTier's "max" branch calls canonTier(env.Max), which
//	returns "" for "top", silently falling back to the phase's base tier
//	instead of recommending "top".
//
// AC map:
//
//	AC-1 sanitizeAdvisorTier accepts "top" (advisor half)       -> C517_001 (behavioral; pre-existing GREEN, cycle-516 landed)
//	AC-2 policy.TierRank classifies "top" as rank 4              -> C517_002 (behavioral; pre-existing GREEN, cycle-516 landed)
//	AC-3 canonTier round-trips "top" (tierFromRank rank-4 gap)   -> C517_003 (behavioral, RED)
//	AC-4 "up" bias strategy can reach "top"                      -> C517_004 (behavioral, RED)
//	AC-5 clamping UP to a "top" floor does not degenerate to ""  -> C517_005 (behavioral, negative, RED)
//	AC-6 max-quality preset end-to-end recommends "top"           -> C517_006 (behavioral, RED)
//	AC-7 tierModelsFor surfaces a "top" key (identity fallback)  -> C517_007 (behavioral, RED)
//	AC-8 go vet ./go/..., existing setup/policy/core suites stay -> manual+checklist (Auditor):
//	     green, apicover -enforce clean on touched packages          run `go vet ./go/...`;
//	     (go/internal/setup)                                         `go test ./go/internal/setup/... ./go/internal/policy/... ./go/internal/core/...`;
//	                                                                  `apicover -enforce` scoped to go/internal/setup
//
// 1:1 enforcement: 8 total ACs = 7 predicate + 1 manual+checklist + 0
// unverifiable-remove.
//
// Predicate strategy (mirrors cycle503/cycle507/cycle514): BEHAVIORAL
// predicates drive the system under test through its in-package RED tests via
// subprocess `go test`, asserting a non-degenerate pass (requireTestsRan
// closes the cycle-85 "no tests to run" trap) — never a source grep. The
// in-package tests were authored by the TDD engineer:
//
//	internal/core/phase_advisor_tier_test.go   (AC-1, pre-existing from cycle 516)
//	internal/policy/policy_test.go             (AC-2, pre-existing from cycle 516)
//	internal/setup/recommend_tier_top_test.go  (AC-3..AC-7, new this cycle)
//
// The Builder implements production code ONLY (tierFromRank + biasTier's "up"
// numeric cap in recommend.go; abstractTiers in setup.go); it must not modify
// the tests.
package cycle517

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const setupPkg = "github.com/mickeyyaya/evolve-loop/go/internal/setup"

// runGoTest runs `go test` on pkg filtered by runFilter, returning combined
// output + exit code. Behavioral predicates invoke the system under test
// through its own in-package tests — no source-grep gaming.
func runGoTest(t *testing.T, runFilter, pkg string) (out string, code int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", runFilter, pkg)
	return stdout + "\n" + stderr, code
}

// requireTestsRan closes the degenerate-predicate trap: `go test -run X` with
// no matching test (renamed/unwritten) — or a package that fails to build —
// exits without running the required tests, which must NOT green the
// predicate.
func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten or renamed")
		return
	}
	if got := strings.Count(out, "=== RUN"); got < min {
		t.Errorf("only %d test(s) ran, need >= %d (package build failure or renamed tests)", got, min)
	}
}

// TestC517_001_AdvisorSanitizerAcceptsTop (AC-1, positive, pre-existing
// GREEN): sanitizeAdvisorTier must pass "top" through unchanged (cycle-516
// landed this). Pinned as a regression guard so this cycle's setup-package
// fix cannot silently coincide with an advisor-side regression.
func TestC517_001_AdvisorSanitizerAcceptsTop(t *testing.T) {
	out, code := runGoTest(t, "TestSanitizeAdvisorTier", "github.com/mickeyyaya/evolve-loop/go/internal/core")
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("sanitizeAdvisorTier regressed on the \"top\" tier (exit=%d)\n%s", code, out)
	}
}

// TestC517_002_PolicyTierRankClassifiesTop (AC-2, positive, pre-existing
// GREEN): policy.TierRank must classify "top" as rank 4 (cycle-516 landed
// this). Pinned as a regression guard — every fix in this cycle builds
// directly on this rank.
func TestC517_002_PolicyTierRankClassifiesTop(t *testing.T) {
	out, code := runGoTest(t, "TestTierRank", "github.com/mickeyyaya/evolve-loop/go/internal/policy")
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("policy.TierRank regressed on the \"top\" tier (exit=%d)\n%s", code, out)
	}
}

// TestC517_003_CanonTierRoundTripsTop (AC-3, positive, RED): canonTier must
// round-trip "top" — the setup package's own reverse-mapping of
// policy.TierRank was never extended to rank 4. Drives internal/setup
// TestCanonTier_TopPassesThrough.
func TestC517_003_CanonTierRoundTripsTop(t *testing.T) {
	out, code := runGoTest(t, "TestCanonTier_TopPassesThrough", setupPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("canonTier(\"top\") does not round-trip (exit=%d) — tierFromRank must map rank 4 back to \"top\"\n%s", code, out)
	}
}

// TestC517_004_UpBiasReachesTop (AC-4, positive, RED): the generic "up" bias
// strategy must be able to climb to "top" when the envelope allows it.
// Drives internal/setup TestBiasTier_UpBias_ReachesTopWhenEnvelopeAllows.
func TestC517_004_UpBiasReachesTop(t *testing.T) {
	out, code := runGoTest(t, "TestBiasTier_UpBias_ReachesTopWhenEnvelopeAllows", setupPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("biasTier(\"up\", ...) cannot reach \"top\" (exit=%d) — its numeric rank cap must extend to 4\n%s", code, out)
	}
}

// TestC517_005_ClampToTopFloorNotEmpty (AC-5, negative, RED): clamping UP to a
// "top" envelope floor must not degenerate to the empty string. Drives
// internal/setup TestClampTier_EnvelopeMinTop_ClampsUpToTopNotEmpty.
func TestC517_005_ClampToTopFloorNotEmpty(t *testing.T) {
	out, code := runGoTest(t, "TestClampTier_EnvelopeMinTop_ClampsUpToTopNotEmpty", setupPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("clampTier degenerates a \"top\" floor clamp to \"\" (exit=%d)\n%s", code, out)
	}
}

// TestC517_006_MaxQualityPresetRecommendsTop (AC-6, positive, end-to-end,
// RED): the shipped "max-quality" preset (tier_bias="max") must actually
// recommend "top" for a phase whose envelope allows it — the full Recommend()
// pipeline, not just the unexported helpers in isolation. Drives
// internal/setup TestRecommend_MaxQualityBiasesToTop.
func TestC517_006_MaxQualityPresetRecommendsTop(t *testing.T) {
	out, code := runGoTest(t, "TestRecommend_MaxQualityBiasesToTop", setupPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("the max-quality preset does not recommend \"top\" end-to-end (exit=%d)\n%s", code, out)
	}
}

// TestC517_007_TierModelsForSurfacesTop (AC-7, positive, RED): tierModelsFor
// must surface a "top" key (identity fallback) for every CLI so onboarding
// can document/report it. Drives internal/setup
// TestTierModelsFor_IncludesTopIdentityFallback.
func TestC517_007_TierModelsForSurfacesTop(t *testing.T) {
	out, code := runGoTest(t, "TestTierModelsFor_IncludesTopIdentityFallback", setupPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("tierModelsFor does not surface a \"top\" key (exit=%d) — abstractTiers must include \"top\"\n%s", code, out)
	}
}
