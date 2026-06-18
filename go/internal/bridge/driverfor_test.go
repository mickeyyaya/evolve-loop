package bridge

import "testing"

// TestDriverFor_BareAndDriverNames pins the single-source bare→driver
// projection shared by every dispatch site (subagent + consensusdispatch).
// Bare CLI names map onto their registered driver; an already-registered
// driver name passes through unchanged; an unknown name is returned as-is so
// the caller's LookupDriver miss surfaces the original (unknown) name.
func TestDriverFor_BareAndDriverNames(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// bare names with NO same-name driver → mapped via bareDriverMap
		{"claude", "claude-tmux"},
		{"gemini", "claude-tmux"}, // gemini.sh's HYBRID mode delegated to claude
		// bare names that ARE themselves registered drivers → pass through
		// (the bareDriverMap entries for codex/agy are dormant: the same-name
		// headless driver already serves as a valid dispatch target).
		{"codex", "codex"},
		{"agy", "agy"},
		// already-registered driver names pass through
		{"claude-tmux", "claude-tmux"},
		{"claude-p", "claude-p"},
		{"codex-tmux", "codex-tmux"},
		{"agy-tmux", "agy-tmux"},
		// unknown → returned unchanged (caller's LookupDriver miss reports it)
		{"no-such-cli", "no-such-cli"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := DriverFor(tc.in); got != tc.want {
			t.Errorf("DriverFor(%q)=%q, want %q", tc.in, got, tc.want)
		}
		// Every mapped non-empty result must resolve to a registered driver
		// (except the deliberate unknown/empty pass-throughs).
		if tc.in != "no-such-cli" && tc.in != "" {
			if _, ok := LookupDriver(DriverFor(tc.in)); !ok {
				t.Errorf("DriverFor(%q)=%q is not a registered driver", tc.in, tc.want)
			}
		}
	}
}
