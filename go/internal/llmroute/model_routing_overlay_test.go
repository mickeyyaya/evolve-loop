package llmroute

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// TestApplySoftOverlay_ZeroValueIsNoop (mr4-projection AC3, I8 byte-identical
// regression floor): a zero-value Overlay ({CLI:"",Tier:""} — the static/
// advisory no-proposal case) returns the input Plan unchanged.
func TestApplySoftOverlay_ZeroValueIsNoop(t *testing.T) {
	in := Plan{Candidates: []string{"claude-tmux", "codex-tmux"}, Model: "sonnet", Triggers: []int{80, 81}}
	out := ApplySoftOverlay(in, Overlay{}, nil)
	if len(out.Candidates) != 2 || out.Candidates[0] != "claude-tmux" || out.Candidates[1] != "codex-tmux" {
		t.Errorf("Candidates = %v, want unchanged [claude-tmux codex-tmux]", out.Candidates)
	}
	if out.Model != "sonnet" {
		t.Errorf("Model = %q, want unchanged sonnet", out.Model)
	}
}

// TestApplySoftOverlay_CLIPromotedToPrimaryChainPreserved (mr4-projection
// AC1/AC5, I3): a non-empty overlay.CLI becomes the chain PRIMARY, but every
// existing candidate — including the pre-overlay primary — survives in the
// chain (deduped, order preserved). This is what distinguishes a SOFT
// overlay from a HARD pin: the chain never shrinks to a single candidate.
func TestApplySoftOverlay_CLIPromotedToPrimaryChainPreserved(t *testing.T) {
	in := Plan{Candidates: []string{"claude-tmux", "codex-tmux"}, Triggers: []int{80, 81}}
	out := ApplySoftOverlay(in, Overlay{CLI: "codex"}, nil)
	if len(out.Candidates) != 2 {
		t.Fatalf("Candidates = %v, want 2 entries (overlay primary + preserved chain, deduped)", out.Candidates)
	}
	if out.Candidates[0] != "codex-tmux" {
		t.Errorf("primary = %q, want codex-tmux (bare family normalized via defaultDriverForFamily, mirroring the pin path)", out.Candidates[0])
	}
	found := map[string]bool{}
	for _, c := range out.Candidates {
		found[c] = true
	}
	if !found["claude-tmux"] {
		t.Errorf("Candidates = %v, want claude-tmux still present (soft overlay never drops a fallback candidate)", out.Candidates)
	}
}

// TestApplySoftOverlay_CLIAlreadyPrimaryDeduped: when the overlay CLI is
// already the plan's primary, promoting it must not duplicate it in the
// chain.
func TestApplySoftOverlay_CLIAlreadyPrimaryDeduped(t *testing.T) {
	in := Plan{Candidates: []string{"claude-tmux", "codex-tmux"}}
	out := ApplySoftOverlay(in, Overlay{CLI: "claude"}, nil)
	if len(out.Candidates) != 2 {
		t.Fatalf("Candidates = %v, want exactly 2 (no duplicate claude-tmux)", out.Candidates)
	}
	if out.Candidates[0] != "claude-tmux" {
		t.Errorf("primary = %q, want claude-tmux", out.Candidates[0])
	}
}

// TestApplySoftOverlay_TierReplacesModel (mr4-projection AC1): a non-empty
// overlay.Tier replaces plan.Model outright (no catalog translation here —
// that happens later at bridge dispatch via the manifest's ModelTierMap).
func TestApplySoftOverlay_TierReplacesModel(t *testing.T) {
	in := Plan{Candidates: []string{"claude-tmux"}, Model: "sonnet"}
	out := ApplySoftOverlay(in, Overlay{Tier: "deep"}, nil)
	if out.Model != "deep" {
		t.Errorf("Model = %q, want deep (overlay tier replaces the resolved model)", out.Model)
	}
}

// TestApplySoftOverlay_TierChainHonorsPhaseEnvelopeFloor (WS-876): an overlaid
// tier's fallback chain must NOT step below the phase's OWN envelope Min under a
// quota wall. A phase declaring min:deep (auditor.json/intent.json) must floor
// the chain at deep, not the universal balanced — else an overlaid auditor would
// silently drop below its configured quality floor when its tier is fully walled.
func TestApplySoftOverlay_TierChainHonorsPhaseEnvelopeFloor(t *testing.T) {
	in := Plan{Candidates: []string{"claude-tmux"}, Model: "sonnet"}
	prof := &profiles.Profile{ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "deep"}}
	out := ApplySoftOverlay(in, Overlay{Tier: "top"}, prof)

	if len(out.Tiers) == 0 || out.Tiers[0] != "top" {
		t.Fatalf("Tiers = %v, want to START at the overlay tier top", out.Tiers)
	}
	deepRank := policy.TierRank("deep")
	for _, tr := range out.Tiers {
		if policy.TierRank(tr) < deepRank {
			t.Errorf("overlaid Tiers %v stepped BELOW the phase envelope floor deep (tier %q rank %d < %d)",
				out.Tiers, tr, policy.TierRank(tr), deepRank)
		}
	}

	// Contrast: with NO profile the universal balanced floor applies, so the
	// chain is allowed to step down to balanced (proving the floor is what changed).
	uni := ApplySoftOverlay(in, Overlay{Tier: "top"}, nil)
	sawBalanced := false
	for _, tr := range uni.Tiers {
		if tr == "balanced" {
			sawBalanced = true
		}
	}
	if !sawBalanced {
		t.Errorf("nil-profile Tiers %v should include the universal balanced floor", uni.Tiers)
	}
}

// TestApplySoftOverlay_PureDoesNotMutateInput: ApplySoftOverlay returns a NEW
// Plan; the input plan.Candidates slice is never mutated in place.
func TestApplySoftOverlay_PureDoesNotMutateInput(t *testing.T) {
	inCandidates := []string{"claude-tmux", "codex-tmux"}
	in := Plan{Candidates: inCandidates}
	_ = ApplySoftOverlay(in, Overlay{CLI: "codex"}, nil)
	if inCandidates[0] != "claude-tmux" || inCandidates[1] != "codex-tmux" {
		t.Errorf("input Candidates mutated in place: %v", inCandidates)
	}
}
