package llmroute

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestAllowedDiscovered_EmptyAllowlistPermitsAll(t *testing.T) {
	got := AllowedDiscovered([]string{"agy-tmux", "codex-tmux"}, &profiles.Profile{})
	if len(got) != 2 {
		t.Fatalf("empty allowlist must permit every discovered CLI, got %v", got)
	}
	if got := AllowedDiscovered([]string{"agy-tmux"}, nil); len(got) != 1 {
		t.Fatalf("nil profile must permit all, got %v", got)
	}
}

func TestAllowedDiscovered_FiltersByFamily(t *testing.T) {
	prof := &profiles.Profile{AllowedCLIs: []string{"claude"}}
	if got := AllowedDiscovered([]string{"agy-tmux", "codex-tmux"}, prof); len(got) != 0 {
		t.Fatalf("a claude-only phase must never be routed to agy/codex by discovery, got %v", got)
	}
	if got := AllowedDiscovered([]string{"agy-tmux", "claude-tmux"}, prof); len(got) != 1 || got[0] != "claude-tmux" {
		t.Fatalf("allowed family must survive the filter, got %v", got)
	}
}

func TestAllowedDiscovered_WildcardAllPermitsAll(t *testing.T) {
	prof := &profiles.Profile{AllowedCLIs: []string{"all"}}
	got := AllowedDiscovered([]string{"agy-tmux", "codex-tmux"}, prof)
	if len(got) != 2 {
		t.Fatalf(`allowed_clis:["all"] must permit every discovered CLI, got %v`, got)
	}
}

func TestAllowedDiscovered_MultiFamilyAllowlist(t *testing.T) {
	prof := &profiles.Profile{AllowedCLIs: []string{"claude", "codex"}}
	got := AllowedDiscovered([]string{"agy-tmux", "codex-tmux", "ollama-tmux"}, prof)
	if len(got) != 1 || got[0] != "codex-tmux" {
		t.Fatalf("multi-family allowlist must keep only permitted families, got %v", got)
	}
}
