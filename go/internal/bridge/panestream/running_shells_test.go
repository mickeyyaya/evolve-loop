package panestream

import (
	"strings"
	"testing"
)

func TestRunningShells_ReadsTheCountFromTheFooter(t *testing.T) {
	t.Parallel()
	farAbove := strings.Repeat("● working on it\n", 10)
	shells := PaneProfile{ShellCountRegex: `·\s*(?P<count>\d+)\s+shells?\b`}
	cases := []struct {
		name string
		pane string
		p    PaneProfile
		want int
	}{
		{"one shell", "● working\n❯\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · 1 shell\n", shells, 1},
		{"several shells with a hint", "● working\n❯\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · 3 shells · ↓ to manage\n", shells, 3},
		{"no shell in the footer", "● working\n❯\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · ← for agents\n", shells, 0},
		{"zero shells", "● working\n❯\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · 0 shells\n", shells, 0},
		{"the word shell with no count", "● working\n❯\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · shell\n", shells, 0},
		{"shells mentioned far above the footer", farAbove + "● I ran 4 shells earlier\n" + farAbove + "❯\n  ⏵⏵ bypass permissions on (shift+tab to cycle)\n", shells, 0},
		{"a count too large for an int", "● working\n❯\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · 99999999999999999999 shells\n", shells, 0},
		{"empty pane", "", shells, 0},
		{"an empty pattern reads zero", "● working\n❯\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · 3 shells\n", PaneProfile{}, 0},
		{"a pattern with no count group reads zero", "● go test\n>\n? for shortcuts · 2 tasks\n", PaneProfile{ShellCountRegex: `· (\d+) tasks`}, 0},
		{"the count group is read by name", "● go test\n>\n? for shortcuts · v9 · 2 tasks\n", PaneProfile{ShellCountRegex: `v(\d) · (?P<count>\d+) tasks`}, 2},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := RunningShells(tc.pane, tc.p); got != tc.want {
				t.Errorf("RunningShells = %d, want %d for pane:\n%s", got, tc.want, tc.pane)
			}
		})
	}
}
