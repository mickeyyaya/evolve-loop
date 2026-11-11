package bridge

import "testing"

func TestUsageModelMatches_ReadsTheManifestMapAndRejectsUnmappedWords(t *testing.T) {
	t.Parallel()
	cases := []struct {
		cli, word, model string
		want             bool
	}{
		{"claude-tmux", "Opus", "claude-opus-5-5", true},
		{"claude-tmux", "Opus", "claude-sonnet-5-5", false},
		{"claude-tmux", "Fable", "claude-fable-5-1", false},
		{"claude-tmux", "Opus", "", false},
		{"no-such-driver", "Opus", "claude-opus-5-5", false},
	}
	for _, c := range cases {
		if got := UsageModelMatches(c.cli, c.word, c.model); got != c.want {
			t.Errorf("UsageModelMatches(%q, %q, %q) = %v, want %v", c.cli, c.word, c.model, got, c.want)
		}
	}
}

func TestUsageModelFor_ReturnsTheTierModelTheWordMapsTo(t *testing.T) {
	t.Parallel()
	model, ok := UsageModelFor("claude-tmux", "Opus")
	if !ok || !UsageModelMatches("claude-tmux", "Opus", model) {
		t.Fatalf("UsageModelFor(claude-tmux, Opus) = %q, %v; want a model the Opus word matches", model, ok)
	}
	if model, ok := UsageModelFor("claude-tmux", "Fable"); ok {
		t.Errorf("UsageModelFor(claude-tmux, Fable) = %q, true; want no model for an unmapped word", model)
	}
	if _, ok := UsageModelFor("no-such-driver", "Opus"); ok {
		t.Error("UsageModelFor on an unknown driver returned a model")
	}
}

func TestUsageModelMapped_ReportsWhetherWordMatchesAnyManifestTier(t *testing.T) {
	t.Parallel()
	if !UsageModelMapped("claude", "Opus") {
		t.Error("UsageModelMapped(claude, Opus) = false, want true")
	}
	if UsageModelMapped("claude", "Fable") {
		t.Error("UsageModelMapped(claude, Fable) = true, want false")
	}
	if UsageModelMapped("no-such-driver", "Opus") {
		t.Error("UsageModelMapped(no-such-driver, Opus) = true, want false")
	}
}
