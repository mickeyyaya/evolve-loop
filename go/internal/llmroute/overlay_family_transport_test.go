package llmroute

// overlay_family_transport_test.go — RED contract for the family-name
// transport ambiguity (inbox overlay-family-name-transport-ambiguity 0.87;
// hotter since #430 made escalation overlays live-fire). PR #390's
// exact-chain-match rung closed the observed instance; a bare FAMILY the
// chain holds only under a NON-default driver still crossed transport:
// chain [claude-p codex] + overlay "claude" found no exact match, fell to
// defaultDriverForFamily("claude") = claude-tmux, and moved a
// headless-configured phase onto tmux. The decided semantics (documented in
// ApplySoftOverlay): a BARE name (no hyphen) is a FAMILY selector satisfied
// by promoting the chain's existing same-family entry (transport preserved);
// a hyphen-QUALIFIED name is a DRIVER selector that wins even over a
// same-family chain entry (an explicit transport request is never rewritten).

import "testing"

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

// First-match tie-break (diff-review MEDIUM): with several same-family
// entries, chain ORDER is the phase's resolved preference — the rung promotes
// the first, never re-sorts transports.
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
	// "codex" is both a family name and a registered driver; the #390 rung
	// (exact chain entry) outranks everything.
	in := Plan{Candidates: []string{"claude-p", "codex"}}
	out := ApplySoftOverlay(in, Overlay{CLI: "codex"}, nil)
	if out.Candidates[0] != "codex" {
		t.Fatalf("primary = %q, want the chain's exact codex entry (PR #390 rung)", out.Candidates[0])
	}
}
