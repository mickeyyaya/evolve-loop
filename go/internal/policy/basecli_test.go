package policy

import "testing"

// TestBaseCLI_StripsKnownDriverSuffixes (mr4b AC2/AC3): policy.BaseCLI is the
// single exported base-name normalizer — this test binds the exported symbol
// name directly (apicover -enforce naming floor) and pins its documented
// semantics (cycle-440 api-contract §1): sequential-strip of "-tmux" then
// "-p", with a leading TrimSpace, adopted verbatim from the pre-existing
// unexported policy.baseCLI (the more-exercised of the two duplicated
// implementations this task consolidates — see api-contract "Resolved
// ambiguity").
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
		{"claude", "claude"},          // already bare — unchanged
		{"", ""},                      // total function: empty in, empty out
		{"  claude-tmux  ", "claude"}, // leading/trailing whitespace trimmed
		{"codex-tmux-p", "codex"},     // sequential strip (never occurs in practice; pins the resolved ambiguity)
	}
	for _, tc := range cases {
		if got := BaseCLI(tc.in); got != tc.want {
			t.Errorf("BaseCLI(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestBaseCLI_UnrecognizedSuffixUnchanged (edge/OOD): a name that carries
// neither a "-tmux" nor a "-p" suffix — including one with an unrelated
// hyphenated suffix — passes through unchanged (after TrimSpace only). The
// normalization must not over-match arbitrary hyphenated names.
func TestBaseCLI_UnrecognizedSuffixUnchanged(t *testing.T) {
	cases := []string{"mallory-cli", "custom-driver-v2", "claude-headless"}
	for _, in := range cases {
		if got := BaseCLI(in); got != in {
			t.Errorf("BaseCLI(%q) = %q, want unchanged %q (no recognized driver suffix)", in, got, in)
		}
	}
}
