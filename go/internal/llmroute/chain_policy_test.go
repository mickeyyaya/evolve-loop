package llmroute

import (
	"strings"
	"testing"
)

func TestExcludeFamilies_DropsTheOperatorBannedFamilies(t *testing.T) {
	got := ExcludeFamilies([]string{"claude-tmux", "agy-tmux", "codex-tmux", "agy"}, []string{"agy"})
	if strings.Join(got, " ") != "claude-tmux codex-tmux" {
		t.Fatalf("ExcludeFamilies = %v", got)
	}
	if got := ExcludeFamilies([]string{"claude-tmux"}, nil); len(got) != 1 {
		t.Fatalf("no ban keeps all: %v", got)
	}
}

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
