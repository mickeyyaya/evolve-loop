package recipe

import "testing"

func TestLastLines(t *testing.T) {
	cases := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"fewer than n", "a\nb", 5, "a\nb"},
		{"exactly n", "a\nb\nc", 3, "a\nb\nc"},
		{"more than n", "a\nb\nc\nd", 2, "c\nd"},
		{"trailing newline trimmed", "a\nb\n", 2, "a\nb"},
		{"empty", "", 3, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := lastLines(tc.in, tc.n); got != tc.want {
				t.Errorf("lastLines(%q,%d)=%q want %q", tc.in, tc.n, got, tc.want)
			}
		})
	}
}
