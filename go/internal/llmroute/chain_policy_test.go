package llmroute

import (
	"strings"
	"testing"
)

// TestExcludeFamilies_DropsTheOperatorBannedFamilies — the last-resort tail
// honours the operator's family ban (policy workflow.universal_fallback_exclude,
// default ["agy"]: the 2026-06-07 judgment that an error-prone model is the
// worst rescue choice). A banned family may still be a configured primary.
func TestExcludeFamilies_DropsTheOperatorBannedFamilies(t *testing.T) {
	got := ExcludeFamilies([]string{"claude-tmux", "agy-tmux", "codex-tmux", "agy"}, []string{"agy"})
	if strings.Join(got, " ") != "claude-tmux codex-tmux" {
		t.Fatalf("ExcludeFamilies = %v", got)
	}
	if got := ExcludeFamilies([]string{"claude-tmux"}, nil); len(got) != 1 {
		t.Fatalf("no ban keeps all: %v", got)
	}
}

// TestKnownDriver names the registry check the profiles guard uses: a
// cli_fallback entry must be a driver the bridge can launch.
func TestKnownDriver(t *testing.T) {
	for _, d := range []string{"claude-p", "claude-tmux", "codex", "codex-tmux", "agy", "agy-tmux", "ollama-tmux"} {
		if !KnownDriver(d) {
			t.Errorf("%s is registered", d)
		}
	}
	if KnownDriver("gpt-cli") || KnownDriver("") {
		t.Error("unknown names are not drivers")
	}
}
