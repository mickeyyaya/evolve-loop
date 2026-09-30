package setup

import "testing"

func TestCanonTier_TopPassesThrough(t *testing.T) {
	if got := canonTier("top"); got != "top" {
		t.Errorf(`canonTier("top") = %q, want "top" (policy.TierRank already returns 4 for "top" — tierFromRank must map rank 4 back)`, got)
	}
}

func TestBiasTier_UpBias_ReachesTopWhenEnvelopeAllows(t *testing.T) {
	got := biasTier("up", "deep", Envelope{Min: "fast", Default: "deep", Max: "top"})
	if got != "top" {
		t.Errorf(`biasTier("up", "deep", env{max:top}) = %q, want "top" (an envelope ceiling of "top" must let the up-bias climb past deep)`, got)
	}
}

func TestClampTier_EnvelopeMinTop_ClampsUpToTopNotEmpty(t *testing.T) {
	got, clamped := clampTier("deep", Envelope{Min: "top", Max: "top"})
	if !clamped {
		t.Fatalf(`clampTier("deep", env{min:top,max:top}) clamped=false, want true (deep is below the top floor)`)
	}
	if got != "top" {
		t.Errorf(`clampTier("deep", env{min:top,max:top}) = %q, want "top" (clamping UP to a "top" floor must not degenerate to "")`, got)
	}
}

func TestRecommend_MaxQualityBiasesToTop(t *testing.T) {
	rep := mkReport([]CLIStatus{famReady("claude", claudeTM)},
		profilePhase("scout", "claude-tmux", "sonnet", "balanced", "balanced", "top", []string{"all"}, ""),
	)
	if got := asg(t, presetByName(t, Recommend(rep, builtinPresets), "max-quality"), "scout").Tier; got != "top" {
		t.Errorf(`max-quality tier = %q, want "top" (envelope max)`, got)
	}
}

func TestTierModelsFor_TopResolvesToModelNotTierName(t *testing.T) {
	claude := tierModelsFor("claude")
	if claude["top"] != "opus" {
		t.Errorf(`tierModelsFor("claude")["top"] = %q, want "opus" — a family whose manifest declares the tier must resolve a real model id, `+
			`not the tier NAME (claude-tmux.json declares "top": "opus"; echoing "top" here is the value that reaches `+
			`Catalog.Lookup as "resolvable" and the CLI as --model top)`, claude["top"])
	}

	unknown := tierModelsFor("nosuchcli")
	if unknown["top"] != "top" {
		t.Errorf(`tierModelsFor("nosuchcli")["top"] = %q, want "top" (identity fallback — abstractTiers must include "top", `+
			`and a CLI with no manifest has nothing to resolve against)`, unknown["top"])
	}
}
