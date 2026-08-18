package bridge

import "testing"

// TestPendingAtInputLine pins the discrimination the whole fix rests on: a
// re-send fires ONLY when the input line still holds what THIS driver sent.
// The "agent typed something else" row is the double-submit hazard the
// cycle-1526 premise challenge flagged as the highest risk of the fix — a
// driver that re-sends there submits the agent's half-typed text.
func TestPendingAtInputLine(t *testing.T) {
	const nudge = "Please write the deliverable to /ws/build-report.md to complete the phase."
	cases := []struct {
		name   string
		pane   string
		echoes []string
		want   bool
	}{
		{"parked at input line", "● output\n\n❯ " + nudge, []string{nudge}, true},
		{"submitted — line cleared", "● output\n\n❯", []string{nudge}, false},
		{"submitted — echo scrolled above the marker", "❯ " + nudge + "\n● working…\n\n❯", []string{nudge}, false},
		{"agent typed something else — never re-send", "● output\n\n❯ let me check the tests first", []string{nudge}, false},
		{"status chrome below the input line", "❯ " + nudge + "\n  ? for shortcuts", []string{nudge}, true},
		{"pane truncated the echo", "❯ Please write the deliverab", []string{nudge}, true},
		{"paste chip parked", "❯ [Pasted text #1 +812 lines]", []string{"# Evolve Builder", tmuxPastePlaceholderEcho}, true},
		{"no marker on screen at all", "zsh: command not found", []string{nudge}, false},
		{"empty echo never matches", "❯ something", []string{""}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pendingAtInputLine(c.pane, tmuxPromptMarkerDefault, c.echoes); got != c.want {
				t.Errorf("pendingAtInputLine() = %v, want %v\npane:\n%s", got, c.want, c.pane)
			}
		})
	}
}
