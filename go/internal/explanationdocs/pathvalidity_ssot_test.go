package explanationdocs

import "testing"

func TestValidRelative_MatchesCitationStrictness(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"a/../../b", false},
		{"x:y", false},
		{"b\x00d", false},
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
