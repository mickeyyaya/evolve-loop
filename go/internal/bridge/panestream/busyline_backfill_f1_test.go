package panestream

import "testing"

func TestHasBusyLineBackfillF1(t *testing.T) {
	t.Parallel()
	with := busyLineProfile()
	without := with
	without.BusyLineRegex = ""
	cases := []struct {
		name    string
		lines   []string
		profile PaneProfile
		want    bool
	}{
		{"a spinner line among content", []string{"  • 5", "⣽  Editing files...", "done"}, with, true},
		{"the spinner on the first line", []string{"⣾ x"}, with, true},
		{"the spinner on the last line", []string{"a", "b", "⣻ y"}, with, true},
		{"content only, no spinner", []string{"  • 5", "Editing files..."}, with, false},
		{"a spinner glyph with nothing after it is not a busy line", []string{"⣽"}, with, false},
		{"no lines at all", nil, with, false},
		{"an empty slice", []string{}, with, false},
		{"a profile without a busy-line pattern never reports busy", []string{"⣽  Editing files..."}, without, false},
		{"an invalid pattern matches nothing", []string{"⣽  Editing files..."}, PaneProfile{BusyLineRegex: "(["}, false},
	}
	for _, tc := range cases {
		if got := hasBusyLine(tc.lines, tc.profile); got != tc.want {
			t.Errorf("%s: hasBusyLine = %v, want %v", tc.name, got, tc.want)
		}
	}
}
