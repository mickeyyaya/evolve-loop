package explanationdocs

import "testing"

// TestValidRelative_MatchesCitationStrictness pins the single-sourcing of the
// "is this relative path safe?" belief. Before the fix, explanationdocs'
// validRelative and reportdoc's citation validator judged the SAME string
// differently in a subsystem whose purpose is tamper-resistance: validRelative
// accepted uncleaned traversal ("a/../../b"), colons, and NUL bytes that the
// citation reader rejects. The belief now has one home (reportdoc); this test
// pins the strict semantics at the explanationdocs call sites.
func TestValidRelative_MatchesCitationStrictness(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		// The three divergence cases caught by architecture review 2026-09-01:
		{"a/../../b", false}, // cleans to ../b — escapes the root
		{"x:y", false},       // colon — rejected by the citation validator
		{"b\x00d", false},    // NUL byte
		// Agreement cases that must keep working:
		{"", false},
		{".", false},
		{"..", false},
		{"../up", false},
		{"/abs/path", false},
		{"ok/path.md", true},
		{".evolve/runs/cycle-9/explanation.md", true},
	}
	for _, tc := range cases {
		if got := validRelative(tc.path); got != tc.want {
			t.Errorf("validRelative(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
