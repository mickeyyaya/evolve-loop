package bridge

import "testing"

func TestHasToolUse_FollowsTheDriverManifest(t *testing.T) {
	for cli, want := range map[string]bool{
		"ollama-tmux": false,
		"claude-tmux": true,
		"codex-tmux":  true,
		"agy-tmux":    true,
		"nope-tmux":   false,
	} {
		if got := HasToolUse(cli); got != want {
			t.Errorf("HasToolUse(%q) = %v, want %v", cli, got, want)
		}
	}
}
