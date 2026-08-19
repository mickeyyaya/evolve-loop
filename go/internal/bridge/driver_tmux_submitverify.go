package bridge

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// driver_tmux_submitverify.go — every driver-initiated submission verifies
// that it was actually SUBMITTED.
//
// The hole this closes: the tmux REPL driver fired keys with enter=true and
// walked away — the prompt paste (driver_tmux_repl.go, "prompt delivered")
// and the one-shot idle nudge both. In cycles 1505, 1510 and 1517 the nudge
// was still sitting, unsubmitted, at the pane's `❯` input line in the final
// capture, and every nudge record in <phase>-interactions.ndjson read
// "result":"no_effect": the driver had no way to know its own key send did
// nothing. Fire-and-forget is the defect; one capture-and-confirm is the fix.
//
// The pairing matters as much as the re-send. An unconditional second Enter
// would "fix" the stall while re-submitting whatever the agent typed next —
// a worse pane desync than the stall. So a re-send fires ONLY when the input
// line still holds an echo of what THIS driver sent.

const (
	// submitVerifyMaxResends bounds the re-send loop: a pane whose input line
	// never clears is wedged, and hammering it is not a recovery strategy. On
	// exhaustion the driver gives up loudly and the normal stop-review /
	// artifact-timeout path runs.
	submitVerifyMaxResends = 3
	// submitVerifyEchoRunes is how much of the sent text must still be visible
	// at the input line to call it unsubmitted. Long enough that unrelated
	// agent typing cannot collide with it.
	submitVerifyEchoRunes = 40
	// submitVerifyMinEchoRunes is the floor BOTH match directions honour: below
	// it a fragment is too generic to identify what this driver sent, and a
	// match would arm re-sends into whatever the agent typed — the double-submit
	// this guard exists to prevent (cycle-1526 audit M1: the forward direction
	// had no floor, so a short first line like `---` matched almost any input
	// line). Single-sourced: the reverse direction used to hardcode this.
	submitVerifyMinEchoRunes = 8
	// submitVerifySettle is the pause between a re-sent Enter and the capture
	// that judges it — the REPL needs a frame to redraw.
	submitVerifySettle = 500 * time.Millisecond
	// tmuxPasteplaceholderEcho is how a REPL that collapses a large paste into
	// a chip renders it at the input line ("[Pasted text #1 +812 lines]").
	// A parked chip is the real-world unsubmitted-prompt shape, since the
	// prompt's own first line is never echoed in that mode.
	tmuxPastePlaceholderEcho = "[Pasted text"
)

// normalizeWS collapses every whitespace run to a single space so a wrapped or
// re-rendered pane line compares equal to the flat string that was sent.
func normalizeWS(s string) string { return strings.Join(strings.Fields(s), " ") }

// echoChunk is the leading submitVerifyEchoRunes runes of s, normalized —
// rune-sliced so a multi-byte boundary is never cut mid-character.
func echoChunk(s string) string {
	r := []rune(normalizeWS(s))
	if len(r) > submitVerifyEchoRunes {
		r = r[:submitVerifyEchoRunes]
	}
	return string(r)
}

// pendingAtInputLine reports whether pane's input line still holds one of the
// echoes — i.e. the keys were typed into the REPL but never submitted.
//
// It reads only the text AFTER the LAST prompt marker: that is the live input
// line. Text the agent already submitted scrolls ABOVE the marker, so a
// successful submission reads as "clear" on the very next capture and no
// re-send is issued. An empty input line, an absent marker, and text that does
// not match anything this driver sent all mean "not pending" — the anti
// double-submit floor.
func pendingAtInputLine(pane, marker string, echoes []string) bool {
	if marker == "" {
		return false
	}
	i := strings.LastIndex(pane, marker)
	if i < 0 {
		return false
	}
	tail := normalizeWS(pane[i+len(marker):])
	if tail == "" {
		return false
	}
	for _, e := range echoes {
		full := normalizeWS(e)
		if full == "" {
			continue
		}
		// Either direction: the pane shows the head of what we sent (normal),
		// or the pane truncated it to a shorter fragment of the same text.
		// Both are floored at submitVerifyMinEchoRunes — a fragment shorter than
		// that identifies nothing, and matching on it re-sends the agent's text.
		if chunk := echoChunk(full); len([]rune(chunk)) >= submitVerifyMinEchoRunes && strings.Contains(tail, chunk) {
			return true
		}
		if len([]rune(tail)) >= submitVerifyMinEchoRunes && strings.Contains(full, tail) {
			return true
		}
	}
	return false
}

// verifySubmitted confirms a submission cleared the input line and, when it
// did not, re-sends a bare Enter — bounded by submitVerifyMaxResends and loud
// on stderr, so an operator reading a stalled cycle's log sees that the driver
// noticed and acted. Returns the number of re-sends issued.
//
// `pane` is the FIRST observation, supplied by the caller: the prompt site
// hands in the post-paste interval baseline the driver already captured, so
// the clean path adds no capture and no fixture-frame drift (the same rule the
// cycle-274 spill check follows). Only a genuinely pending input line — the
// exceptional path — costs a re-capture per re-send.
//
// site names the submission ("prompt", "nudge") in the log line; echoes are
// candidate renderings of what was sent.
func verifySubmitted(ctx context.Context, deps Deps, lp tmuxLaunch, pfx, site, pane string, echoes ...string) int {
	// No input-line marker ⇒ nothing to anchor a match to. Refuse LOUDLY: the
	// alternative is matching against whatever follows a boot/footer marker,
	// which re-sends the agent's own text (cycle-1526 audit — agy's marker is
	// the footer "? for shortcuts").
	if lp.inputLineMarker == "" {
		fmt.Fprintf(deps.Stderr, "%s submit-verify: %s NOT verified — %s declares no input-line marker, "+
			"so a stalled submission here will not be detected or re-sent\n", pfx, site, lp.name)
		return 0
	}
	resends := 0
	for pendingAtInputLine(pane, lp.inputLineMarker, echoes) {
		if resends >= submitVerifyMaxResends {
			fmt.Fprintf(deps.Stderr, "%s submit-verify: %s STILL unsubmitted after %d re-send(s) — giving up; pane looks wedged\n",
				pfx, site, resends)
			return resends
		}
		fmt.Fprintf(deps.Stderr, "%s submit-verify: %s still parked at the `%s` input line — re-sending Enter (%d/%d)\n",
			pfx, site, lp.inputLineMarker, resends+1, submitVerifyMaxResends)
		if err := deps.Tmux.SendKeys(ctx, lp.session, "", true); err != nil {
			// Without this the loop runs to exhaustion and reports "pane looks
			// wedged" — a confident WRONG diagnosis that points an operator (and
			// any log-scraping classifier) at the REPL when tmux is what died.
			fmt.Fprintf(deps.Stderr, "%s submit-verify: %s re-send %d/%d never reached tmux — the session is unreachable, not wedged: %v\n", pfx, site, resends+1, submitVerifyMaxResends, err)
			return resends
		}
		resends++
		deps.Sleep(submitVerifySettle)
		next, err := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
		if err != nil {
			// Never silent: without this line an operator sees a re-send start
			// and nothing after it, and cannot tell a cleared input line from a
			// dead tmux server.
			fmt.Fprintf(deps.Stderr, "%s submit-verify: %s capture failed after re-send %d/%d — "+
				"stopping verification, input-line state unknown: %v\n",
				pfx, site, resends, submitVerifyMaxResends, err)
			return resends
		}
		pane = next
	}
	return resends
}

// promptSubmitEcho is the FORWARD-direction echo for a pasted prompt: the head
// of the whole prompt, whitespace-normalized so a wrapped render compares equal.
//
// Why the whole prompt and not just its first line: the first line is the only
// prompt-derived echo the prompt site had, and a prompt whose first non-empty
// line is short (YAML frontmatter `---`, a stub header) falls under
// submitVerifyMinEchoRunes — that echo would never match and the guard would
// silently miss the cycles 1505/1510/1517 stall it exists to catch.
//
// It is BOUNDED (echoChunk caps at submitVerifyEchoRunes) and is therefore NOT
// a drop-in replacement for the first line in BOTH match directions: the
// reverse direction reads the passed value directly, so a head-only echo would
// narrow it from "any fragment of the first line" to "any fragment of its first
// 40 runes" — a silent detection loss on a REPL that horizontally scrolls its
// input line (ollama readline) rather than wrapping. The prompt site therefore
// passes this AND firstNonEmptyLine: this one keeps the forward direction above
// the floor, the first line preserves the reverse direction's original reach.
// Passing the WHOLE prompt as an echo would be wrong in the other direction —
// reverse would then match any ≥floor phrase the agent typed that appears
// anywhere in the prompt body.
func promptSubmitEcho(prompt string) string { return echoChunk(prompt) }

// firstNonEmptyLine returns the first line of s with content — the line a REPL
// that echoes a paste verbatim shows at its input line, and the reverse
// direction's echo at the prompt site (see promptSubmitEcho).
func firstNonEmptyLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		if strings.TrimSpace(ln) != "" {
			return ln
		}
	}
	return ""
}
