package policy

import "testing"

func TestBaseCLI_StripsKnownDriverSuffixes(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"claude-tmux", "claude"},
		{"claude-p", "claude"},
		{"codex-tmux", "codex"},
		{"agy-tmux", "agy"},
		{"ollama-tmux", "ollama"},
		{"claude", "claude"},
		{"", ""},
		{"  claude-tmux  ", "claude"},
		{"codex-tmux-p", "codex"}, // never occurs in practice; pins the repeated strip
	}
	for _, tc := range cases {
		if got := BaseCLI(tc.in); got != tc.want {
			t.Errorf("BaseCLI(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBaseCLI_UnrecognizedSuffixUnchanged(t *testing.T) {
	cases := []string{"mallory-cli", "custom-driver-v2", "claude-headless"}
	for _, in := range cases {
		if got := BaseCLI(in); got != in {
			t.Errorf("BaseCLI(%q) = %q, want unchanged %q (no recognized driver suffix)", in, got, in)
		}
	}
}
