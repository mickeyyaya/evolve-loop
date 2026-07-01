package llmroute

import "testing"

// TestApplySoftOverlay_ZeroValueIsNoop (mr4-projection AC3, I8 byte-identical
// regression floor): a zero-value Overlay ({CLI:"",Tier:""} — the static/
// advisory no-proposal case) returns the input Plan unchanged.
func TestApplySoftOverlay_ZeroValueIsNoop(t *testing.T) {
	in := Plan{Candidates: []string{"claude-tmux", "codex-tmux"}, Model: "sonnet", Triggers: []int{80, 81}}
	out := ApplySoftOverlay(in, Overlay{})
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
	out := ApplySoftOverlay(in, Overlay{CLI: "codex"})
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
	out := ApplySoftOverlay(in, Overlay{CLI: "claude"})
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
	out := ApplySoftOverlay(in, Overlay{Tier: "deep"})
	if out.Model != "deep" {
		t.Errorf("Model = %q, want deep (overlay tier replaces the resolved model)", out.Model)
	}
}

// TestApplySoftOverlay_PureDoesNotMutateInput: ApplySoftOverlay returns a NEW
// Plan; the input plan.Candidates slice is never mutated in place.
func TestApplySoftOverlay_PureDoesNotMutateInput(t *testing.T) {
	inCandidates := []string{"claude-tmux", "codex-tmux"}
	in := Plan{Candidates: inCandidates}
	_ = ApplySoftOverlay(in, Overlay{CLI: "codex"})
	if inCandidates[0] != "claude-tmux" || inCandidates[1] != "codex-tmux" {
		t.Errorf("input Candidates mutated in place: %v", inCandidates)
	}
}
