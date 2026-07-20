package runner

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// allowedDiscovered is the security gate on universal fallback: discovery may
// only route a phase to a CLI FAMILY its profile allowlist permits, so an
// operator's per-phase pin (e.g. tester allowed_clis=["claude"]) is never
// bypassed by "whatever's installed."

func TestAllowedDiscovered_EmptyAllowlistPermitsAll(t *testing.T) {
	got := allowedDiscovered([]string{"agy-tmux", "codex-tmux"}, &profiles.Profile{})
	if len(got) != 2 {
		t.Fatalf("empty allowlist must permit every discovered CLI, got %v", got)
	}
	// nil profile also permits all (no restriction to enforce).
	if got := allowedDiscovered([]string{"agy-tmux"}, nil); len(got) != 1 {
		t.Fatalf("nil profile must permit all, got %v", got)
	}
}

func TestAllowedDiscovered_FiltersByFamily(t *testing.T) {
	// claude-only phase (the tester/tdd pin): discovered agy/codex are DROPPED.
	prof := &profiles.Profile{AllowedCLIs: []string{"claude"}}
	if got := allowedDiscovered([]string{"agy-tmux", "codex-tmux"}, prof); len(got) != 0 {
		t.Fatalf("a claude-only phase must never be routed to agy/codex by discovery, got %v", got)
	}
	// claude present in discovery + allowed → kept (mapped by family: claude-tmux → claude).
	if got := allowedDiscovered([]string{"agy-tmux", "claude-tmux"}, prof); len(got) != 1 || got[0] != "claude-tmux" {
		t.Fatalf("allowed family must survive the filter, got %v", got)
	}
}

func TestAllowedDiscovered_WildcardAllPermitsAll(t *testing.T) {
	// allowed_clis:["all"] is the wildcard 14 default profiles use — it must
	// permit every discovered family (not be treated as the literal string "all").
	prof := &profiles.Profile{AllowedCLIs: []string{"all"}}
	got := allowedDiscovered([]string{"agy-tmux", "codex-tmux"}, prof)
	if len(got) != 2 {
		t.Fatalf(`allowed_clis:["all"] must permit every discovered CLI, got %v`, got)
	}
}

func TestAllowedDiscovered_MultiFamilyAllowlist(t *testing.T) {
	// builder pin [claude,codex]: agy dropped, codex kept.
	prof := &profiles.Profile{AllowedCLIs: []string{"claude", "codex"}}
	got := allowedDiscovered([]string{"agy-tmux", "codex-tmux", "ollama-tmux"}, prof)
	if len(got) != 1 || got[0] != "codex-tmux" {
		t.Fatalf("multi-family allowlist must keep only permitted families, got %v", got)
	}
}
