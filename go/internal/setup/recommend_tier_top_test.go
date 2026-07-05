package setup

// recommend_tier_top_test.go — RED tests (cycle 517, task
// advisor-tier-vocab-add-top) for the SECOND half of "wire the 'top' model
// tier through advisor + policy rank". Cycle 516 already wired the advisor
// half (sanitizeAdvisorTier, phase_advisor_tier_test.go — pre-existing GREEN,
// verified this cycle) and policy.TierRank itself already classifies "top" as
// rank 4 (policy_test.go TestTierRank — pre-existing GREEN). The remaining
// gap: package setup's own CONSUMERS of that policy rank never learned about
// rank 4.
//
//   - tierFromRank (recommend.go) only maps ranks 1-3 back to a tier string;
//     rank 4 ("top") falls through to "", so canonTier("top") == "" — the
//     setup/recommend flow cannot even round-trip the literal string "top".
//   - biasTier's "up" strategy hard-caps at `if r < 3 { r++ }`, so a
//     max-quality preset can never recommend "top" even when a phase's
//     envelope explicitly allows it.
//   - clampTier's floor-clamp (`rWant < rMin`) calls tierFromRank(rMin) to
//     produce the clamped value; when rMin is 4 ("top"), the same gap
//     silently downgrades a valid clamp target to the empty string — a
//     broken tier is worse than no clamp.
//   - abstractTiers (setup.go) is still the pre-4-tier {fast,balanced,deep}
//     literal, so tierModelsFor never surfaces a "top" key in CLIStatus.
//     TierModels at all (onboarding can never document/report it).
//
// RED today: every test below fails against the current 3-tier-only
// implementation. Do NOT modify this file — implement the seam.

import "testing"

// TestCanonTier_TopPassesThrough (AC, positive core case): canonTier must
// round-trip the literal "top" — the SAME canonical string policy.TierRank
// already classifies as rank 4 must not be silently dropped back to "".
func TestCanonTier_TopPassesThrough(t *testing.T) {
	if got := canonTier("top"); got != "top" {
		t.Errorf(`canonTier("top") = %q, want "top" (policy.TierRank already returns 4 for "top" — tierFromRank must map rank 4 back)`, got)
	}
}

// TestBiasTier_UpBias_ReachesTopWhenEnvelopeAllows (AC, positive): the
// generic "up" bias strategy (available to any custom preset config via
// knownTierBias — presets.json's builtin "max-quality" actually uses the
// "max" strategy, covered end-to-end by TestRecommend_MaxQualityBiasesToTop)
// must be able to reach "top" from "deep" when nothing in the envelope
// forbids it — the one-rank-higher counterpart of
// TestRecommend_MaxQualityBiasesUp's deep-ceiling case.
func TestBiasTier_UpBias_ReachesTopWhenEnvelopeAllows(t *testing.T) {
	got := biasTier("up", "deep", Envelope{Min: "fast", Default: "deep", Max: "top"})
	if got != "top" {
		t.Errorf(`biasTier("up", "deep", env{max:top}) = %q, want "top" (an envelope ceiling of "top" must let the up-bias climb past deep)`, got)
	}
}

// TestClampTier_EnvelopeMinTop_ClampsUpToTopNotEmpty (AC, negative /
// anti-degenerate): when a phase's envelope FLOOR is "top" (an operator
// pinning a phase to the frontier tier only), clampTier must clamp a lower
// request UP to "top" — not to the empty string. A broken clamp target is
// worse than no clamp: it would make the phase's tier unclassifiable
// downstream instead of correctly frontier-only.
func TestClampTier_EnvelopeMinTop_ClampsUpToTopNotEmpty(t *testing.T) {
	got, clamped := clampTier("deep", Envelope{Min: "top", Max: "top"})
	if !clamped {
		t.Fatalf(`clampTier("deep", env{min:top,max:top}) clamped=false, want true (deep is below the top floor)`)
	}
	if got != "top" {
		t.Errorf(`clampTier("deep", env{min:top,max:top}) = %q, want "top" (clamping UP to a "top" floor must not degenerate to "")`, got)
	}
}

// TestRecommend_MaxQualityBiasesToTop (AC, end-to-end wiring): a phase whose
// profile envelope allows up to "top" must actually be RECOMMENDED "top" by
// the max-quality preset — the full Recommend() pipeline, not just the
// unexported helpers in isolation. This is the anti-dead-code assertion: a
// tierFromRank fix that only unit tests pass here would still fail if some
// OTHER hardcoded 3-rank assumption remains on the path Recommend actually
// walks.
func TestRecommend_MaxQualityBiasesToTop(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM)},
		ph("scout", "claude-tmux", "sonnet", "balanced", "balanced", "top", []string{"all"}, ""),
	)
	if got := asg(t, presetByName(t, Recommend(rep, builtinPresets), "max-quality"), "scout").Tier; got != "top" {
		t.Errorf(`max-quality tier = %q, want "top" (envelope max)`, got)
	}
}

// TestTierModelsFor_IncludesTopIdentityFallback (AC, positive): tierModelsFor
// must resolve a "top" entry for every CLI, even before any bridge manifest
// declares a native model for it — the SAME identity-fallback contract
// fast/balanced/deep already get when a manifest lacks the entry (claude's
// tier_aliases are empty, so its map is pure identity — the exact fixture
// TestTierModelsFor already uses for fast/balanced/deep).
func TestTierModelsFor_IncludesTopIdentityFallback(t *testing.T) {
	t.Setenv("EVOLVE_MODEL_CATALOG_DIR", t.TempDir())
	claude := tierModelsFor("claude")
	if claude["top"] != "top" {
		t.Errorf(`tierModelsFor("claude")["top"] = %q, want "top" (identity fallback — abstractTiers must include "top")`, claude["top"])
	}
}
