package bridge

import (
	"strings"
	"testing"
)

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
		// cycle-1526 audit M1: the forward branch had no length floor while the
		// reverse branch enforced one, so a SHORT echo matched almost any input
		// line — arming up to submitVerifyMaxResends Enters into text the AGENT
		// typed. That is the double-submit desync this guard exists to prevent.
		// A prompt whose first non-empty line is short (frontmatter `---`, a
		// stub header) is all it takes; today's 53-char first lines are a
		// measurement, not an invariant.
		{"short echo must not match agent typing", "❯ let me check --- the diff", []string{"---"}, false},
		{"sub-floor echo below the rune floor", "❯ yes", []string{"y"}, false},
		{"echo one rune below the floor", "❯ abcdefg and more", []string{"abcdefg"}, false},
		{"echo exactly at the rune floor still matches", "❯ abcdefgh and more", []string{"abcdefgh"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pendingAtInputLine(c.pane, tmuxPromptMarkerDefault, c.echoes); got != c.want {
				t.Errorf("pendingAtInputLine() = %v, want %v\npane:\n%s", got, c.want, c.pane)
			}
		})
	}

	// Second mechanism, pinned separately (review M1): a family that declares no
	// input-line marker is refused HERE too, not only by verifySubmitted's early
	// return. The two are deliberately redundant — that is why a mutation which
	// deletes the early return alone is still safe — so the redundancy itself
	// needs a test, or a later "simplification" could remove this check and
	// leave the whole guard resting on one branch.
	t.Run("empty marker is refused — no anchor, no match", func(t *testing.T) {
		if pendingAtInputLine("● out\n? for shortcuts\n"+nudge, "", []string{nudge}) {
			t.Error("pendingAtInputLine() = true for an empty marker; with nothing to anchor on it must never claim the input line is pending")
		}
	})
}

// TestPromptSubmitEcho pins the prompt-site echo pair. Both cycle-1526 reviewers
// observed that reverting promptSubmitEcho to firstNonEmptyLine left the whole
// suite green — the helper's entire reason for existing was asserted in a
// comment and never demonstrated. These rows fail for that revert, and for the
// opposite degenerate form (returning the prompt unbounded).
func TestPromptSubmitEcho(t *testing.T) {
	// The motivating shape: a short first non-empty line under the rune floor.
	frontmatter := "---\ntitle: build\n---\n\n## Task\nRefactor the widget pipeline end to end.\n"

	t.Run("short first line still yields an echo at or above the floor", func(t *testing.T) {
		got := promptSubmitEcho(frontmatter)
		if n := len([]rune(got)); n < submitVerifyMinEchoRunes {
			t.Fatalf("echo %q is %d runes, below the %d floor — it can never match, so a parked "+
				"prompt goes undetected (the stall this helper exists to catch)", got, n, submitVerifyMinEchoRunes)
		}
		// And it must actually detect the parked prompt end-to-end.
		pane := "● scrollback\n\n" + tmuxPromptMarkerDefault + " " + normalizeWS(frontmatter)
		if !pendingAtInputLine(pane, tmuxPromptMarkerDefault, []string{promptSubmitEcho(frontmatter)}) {
			t.Error("parked frontmatter prompt not detected — firstNonEmptyLine would yield \"---\" (3 runes), below the floor")
		}
	})

	t.Run("bounded: never wider than the echo window", func(t *testing.T) {
		huge := strings.Repeat("word ", 4000)
		if n := len([]rune(promptSubmitEcho(huge))); n != submitVerifyEchoRunes {
			t.Errorf("echo is %d runes, want exactly %d — an unbounded echo lets the reverse direction "+
				"match any phrase appearing anywhere in the prompt body", n, submitVerifyEchoRunes)
		}
	})

	t.Run("a phrase deep in the prompt body never arms a re-send", func(t *testing.T) {
		prompt := "## Subagent Interactive Policy (recommended_or_first)\n" +
			strings.Repeat("filler line of prose here\n", 20) + "\nDELETE THE PRODUCTION DATABASE\n"
		pane := "● out\n\n" + tmuxPromptMarkerDefault + " DELETE THE PRODUCTION DATABASE"
		if pendingAtInputLine(pane, tmuxPromptMarkerDefault, []string{promptSubmitEcho(prompt)}) {
			t.Error("a body phrase the AGENT typed matched the prompt echo — that re-sends the agent's text")
		}
	})

	t.Run("long first line: forward reach preserved", func(t *testing.T) {
		real := "## Subagent Interactive Policy (recommended_or_first)\nrest of prompt\n"
		pane := "● out\n\n" + tmuxPromptMarkerDefault + " " + real[:45]
		if !pendingAtInputLine(pane, tmuxPromptMarkerDefault, []string{promptSubmitEcho(real)}) {
			t.Error("a parked real prompt head was not detected")
		}
	})
}
