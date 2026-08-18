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
		if strings.Contains(tail, echoChunk(full)) {
			return true
		}
		if len([]rune(tail)) >= 8 && strings.Contains(full, tail) {
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
	resends := 0
	for pendingAtInputLine(pane, lp.promptMarker, echoes) {
		if resends >= submitVerifyMaxResends {
			fmt.Fprintf(deps.Stderr, "%s submit-verify: %s STILL unsubmitted after %d re-send(s) — giving up; pane looks wedged\n",
				pfx, site, resends)
			return resends
		}
		fmt.Fprintf(deps.Stderr, "%s submit-verify: %s still parked at the `%s` input line — re-sending Enter (%d/%d)\n",
			pfx, site, lp.promptMarker, resends+1, submitVerifyMaxResends)
		_ = deps.Tmux.SendKeys(ctx, lp.session, "", true)
		resends++
		deps.Sleep(submitVerifySettle)
		next, err := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
		if err != nil {
			return resends
		}
		pane = next
	}
	return resends
}

// firstNonEmptyLine returns the first line of s with content — the line a REPL
// that echoes a paste verbatim shows at its input line.
func firstNonEmptyLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		if strings.TrimSpace(ln) != "" {
			return ln
		}
	}
	return ""
}
