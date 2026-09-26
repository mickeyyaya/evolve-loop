package llmroute

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

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

func TestApplySoftOverlay_TierReplacesModel(t *testing.T) {
	in := Plan{Candidates: []string{"claude-tmux"}, Model: "sonnet"}
	out := ApplySoftOverlay(in, Overlay{Tier: "deep"}, nil)
	if out.Model != "deep" {
		t.Errorf("Model = %q, want deep (overlay tier replaces the resolved model)", out.Model)
	}
}

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

	// Control: without a profile the universal balanced floor applies.
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

func TestApplySoftOverlay_PureDoesNotMutateInput(t *testing.T) {
	inCandidates := []string{"claude-tmux", "codex-tmux"}
	in := Plan{Candidates: inCandidates}
	_ = ApplySoftOverlay(in, Overlay{CLI: "codex"}, nil)
	if inCandidates[0] != "claude-tmux" || inCandidates[1] != "codex-tmux" {
		t.Errorf("input Candidates mutated in place: %v", inCandidates)
	}
}

func TestApplySoftOverlay_PromotesAnExistingCandidateWithoutRewritingItsTransport(t *testing.T) {
	in := Plan{Candidates: []string{"claude-p", "codex"}, Triggers: []int{80, 81}}
	out := ApplySoftOverlay(in, Overlay{CLI: "codex"}, nil)
	if out.Candidates[0] != "codex" {
		t.Errorf("primary = %q, want codex — the chain already names the headless driver; normalizing it to codex-tmux crosses transport and needs a tmux the host may not have", out.Candidates[0])
	}
	if len(out.Candidates) != 2 {
		t.Errorf("Candidates = %v, want 2 (promotion, not addition)", out.Candidates)
	}
	found := map[string]bool{}
	for _, c := range out.Candidates {
		found[c] = true
	}
	if !found["claude-p"] {
		t.Errorf("Candidates = %v, want claude-p still present (soft overlay never drops a fallback candidate)", out.Candidates)
	}
	if found["codex-tmux"] {
		t.Errorf("Candidates = %v, want NO codex-tmux — a transport the phase never chose", out.Candidates)
	}
}
