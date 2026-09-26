package bridge

import "testing"

func TestDriverFor_BareAndDriverNames(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"claude", "claude-tmux"},
		{"gemini", "claude-tmux"}, // gemini.sh's HYBRID mode delegated to claude
		{"codex", "codex"},
		{"agy", "agy"},
		{"claude-tmux", "claude-tmux"},
		{"claude-p", "claude-p"},
		{"codex-tmux", "codex-tmux"},
		{"agy-tmux", "agy-tmux"},
		{"no-such-cli", "no-such-cli"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := DriverFor(tc.in); got != tc.want {
			t.Errorf("DriverFor(%q)=%q, want %q", tc.in, got, tc.want)
		}
		if tc.in != "no-such-cli" && tc.in != "" {
			if _, ok := LookupDriver(DriverFor(tc.in)); !ok {
				t.Errorf("DriverFor(%q)=%q is not a registered driver", tc.in, tc.want)
			}
		}
	}
}
