package panestream

import (
	"strings"
	"testing"
)

func TestRunningShells_ReadsTheCountFromTheFooter(t *testing.T) {
	t.Parallel()
	farAbove := strings.Repeat("● working on it\n", 10)
	cases := []struct {
		name string
		pane string
		want int
	}{
		{"one shell", "● working\n❯\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · 1 shell\n", 1},
		{"several shells with a hint", "● working\n❯\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · 3 shells · ↓ to manage\n", 3},
		{"no shell in the footer", "● working\n❯\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · ← for agents\n", 0},
		{"zero shells", "● working\n❯\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · 0 shells\n", 0},
		{"the word shell with no count", "● working\n❯\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · shell\n", 0},
		{"a count that is not a number", "● working\n❯\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · many shells\n", 0},
		{"shells mentioned far above the footer", farAbove + "● I ran 4 shells earlier\n" + farAbove + "❯\n  ⏵⏵ bypass permissions on (shift+tab to cycle)\n", 0},
		{"a count too large for an int", "● working\n❯\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · 99999999999999999999 shells\n", 0},
		{"empty pane", "", 0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := RunningShells(tc.pane); got != tc.want {
				t.Errorf("RunningShells = %d, want %d for pane:\n%s", got, tc.want, tc.pane)
			}
		})
	}
}
