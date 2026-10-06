package llmroute

import (
	"slices"
	"testing"
)

func TestApplySoftOverlay_BareFamilyDoesNotCrossTransportWhenTheChainHoldsANonDefaultDriver(t *testing.T) {
	t.Parallel()
	in := Plan{Candidates: []string{"claude-p", "codex"}}
	out := ApplySoftOverlay(in, Overlay{CLI: "claude"}, nil)
	if out.Candidates[0] != "claude-p" {
		t.Fatalf("primary = %q, want claude-p — a bare-family overlay moved a headless phase onto %q (transport cross, the exit=10/no-tmux class)", out.Candidates[0], out.Candidates[0])
	}
}

func TestApplySoftOverlay_DriverQualifiedOverlayWinsOverSameFamilyChainEntry(t *testing.T) {
	t.Parallel()
	in := Plan{Candidates: []string{"claude-p"}}
	out := ApplySoftOverlay(in, Overlay{CLI: "claude-tmux"}, nil)
	if out.Candidates[0] != "claude-tmux" {
		t.Fatalf("primary = %q, want claude-tmux — an EXPLICIT driver-qualified overlay must never be satisfied by promoting a same-family entry with the opposite transport", out.Candidates[0])
	}
	if len(out.Candidates) < 2 || out.Candidates[1] != "claude-p" {
		t.Errorf("the chain's own entry must remain as fallback: %v", out.Candidates)
	}
}

func TestApplySoftOverlay_BareFamilyPromotesFirstSameFamilyEntry(t *testing.T) {
	t.Parallel()
	in := Plan{Candidates: []string{"claude-tmux", "claude-p"}}
	out := ApplySoftOverlay(in, Overlay{CLI: "claude"}, nil)
	if out.Candidates[0] != "claude-tmux" {
		t.Fatalf("primary = %q, want the FIRST same-family chain entry (chain order is the resolved preference)", out.Candidates[0])
	}
}

func TestApplySoftOverlay_BareFamilyWithNoFamilyEntryStillDefaults(t *testing.T) {
	t.Parallel()
	in := Plan{Candidates: []string{"agy-tmux"}}
	out := ApplySoftOverlay(in, Overlay{CLI: "claude"}, nil)
	if out.Candidates[0] != "claude-tmux" {
		t.Fatalf("primary = %q, want claude-tmux — a bare family the chain does not hold keeps the family-default rung", out.Candidates[0])
	}
}

func TestApplySoftOverlay_ExactChainMatchStillWinsForBothNames(t *testing.T) {
	t.Parallel()
	// "codex" is both a family name and a registered driver.
	in := Plan{Candidates: []string{"claude-p", "codex"}}
	out := ApplySoftOverlay(in, Overlay{CLI: "codex"}, nil)
	if out.Candidates[0] != "codex" {
		t.Fatalf("primary = %q, want the chain's exact codex entry (PR #390 rung)", out.Candidates[0])
	}
}

func TestApplySoftOverlay_AnOverlayPromotesTheChainEntryOfItsRoutingFamily(t *testing.T) {
	t.Parallel()
	chain := []string{"codex-tmux", "agy-claude-tmux", "agy-tmux", "claude-tmux"}
	cases := []struct {
		overlay string
		want    []string
	}{
		{"agy", []string{"agy-tmux", "codex-tmux", "agy-claude-tmux", "claude-tmux"}},
		{"agy-claude", []string{"agy-claude-tmux", "codex-tmux", "agy-tmux", "claude-tmux"}},
		{"claude", []string{"claude-tmux", "codex-tmux", "agy-claude-tmux", "agy-tmux"}},
		{"agy-claude-tmux", []string{"agy-claude-tmux", "codex-tmux", "agy-tmux", "claude-tmux"}},
	}
	for _, tc := range cases {
		out := ApplySoftOverlay(Plan{Candidates: chain}, Overlay{CLI: tc.overlay}, nil)
		if !slices.Equal(out.Candidates, tc.want) {
			t.Errorf("overlay %q over %v = %v, want %v", tc.overlay, chain, out.Candidates, tc.want)
		}
	}
}
