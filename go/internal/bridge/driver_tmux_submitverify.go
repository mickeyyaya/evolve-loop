package bridge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

const (
	submitVerifyMaxResends = 3
	// submitVerifyEchoRunes is how much of the sent text must still be visible
	// at the input line to call it unsubmitted. Long enough that unrelated
	// agent typing cannot collide with it.
	submitVerifyEchoRunes = 40
	// submitVerifyMinEchoRunes is the floor BOTH match directions honour: below
	// it a fragment is too generic to identify what this driver sent, and a
	// match would arm re-sends into whatever the agent typed.
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
// echoes — i.e. the keys were typed into the REPL but never submitted. It
// reads only the text AFTER the LAST prompt marker, which is the live input
// line; text already submitted scrolls above the marker. An empty input
// line, an absent marker, or a non-matching echo all mean "not pending".
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
// on stderr. Returns the number of re-sends issued.
//
// `pane` is the FIRST observation, supplied by the caller, so the clean path
// costs no extra capture; only a genuinely pending input line costs a
// re-capture per re-send.
//
// site names the submission ("prompt", "nudge") in the log line; echoes are
// candidate renderings of what was sent.

// submitVerifyOutcome is what verifySubmitted learned, returned so the
// caller can record it durably.
type submitVerifyOutcome struct {
	// Resends is how many bare Enters this call issued (0 on the clean path).
	Resends int
	// Result is an interaction.Result* value; never empty.
	Result string
}

func verifySubmitted(ctx context.Context, deps Deps, lp tmuxLaunch, pfx, site, pane string, echoes ...string) submitVerifyOutcome {
	if lp.inputLineMarker == "" {
		fmt.Fprintf(deps.Stderr, "%s submit-verify: %s NOT verified — %s declares no input-line marker, "+
			"so a stalled submission here will not be detected or re-sent\n", pfx, site, lp.name)
		return submitVerifyOutcome{Result: interaction.ResultNotVerified}
	}
	if strings.TrimSpace(pane) == "" {
		fmt.Fprintf(deps.Stderr, "%s submit-verify: %s NOT verified — no pane observation, "+
			"input-line state unknown\n", pfx, site)
		return submitVerifyOutcome{Result: interaction.ResultNotVerified}
	}
	resends := 0
	for pendingAtInputLine(pane, lp.inputLineMarker, echoes) {
		if resends >= submitVerifyMaxResends {
			fmt.Fprintf(deps.Stderr, "%s submit-verify: %s STILL unsubmitted after %d re-send(s) — giving up; pane looks wedged\n",
				pfx, site, resends)
			return submitVerifyOutcome{Resends: resends, Result: interaction.ResultSubmitWedged}
		}
		fmt.Fprintf(deps.Stderr, "%s submit-verify: %s still parked at the `%s` input line — re-sending Enter (%d/%d)\n",
			pfx, site, lp.inputLineMarker, resends+1, submitVerifyMaxResends)
		if err := deps.Tmux.SendKeys(ctx, lp.session, "", true); err != nil {
			// Without this the loop would exhaust and report "wedged" — a wrong
			// diagnosis when tmux itself, not the REPL, is what died.
			fmt.Fprintf(deps.Stderr, "%s submit-verify: %s re-send %d/%d never reached tmux — the session is unreachable, not wedged: %v\n", pfx, site, resends+1, submitVerifyMaxResends, err)
			return submitVerifyOutcome{Resends: resends, Result: interaction.ResultNotVerified}
		}
		resends++
		deps.Sleep(submitVerifyBackoff[resends-1])
		next, err := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
		if err != nil {
			fmt.Fprintf(deps.Stderr, "%s submit-verify: %s capture failed after re-send %d/%d — "+
				"stopping verification, input-line state unknown: %v\n",
				pfx, site, resends, submitVerifyMaxResends, err)
			return submitVerifyOutcome{Resends: resends, Result: interaction.ResultNotVerified}
		}
		pane = next
	}
	if resends > 0 {
		return submitVerifyOutcome{Resends: resends, Result: interaction.ResultSubmittedAfterResend}
	}
	return submitVerifyOutcome{Result: interaction.ResultSubmitVerified}
}

// promptSubmitEcho is the FORWARD-direction echo for a pasted prompt: the
// bounded head of the whole prompt, whitespace-normalized so a wrapped render
// compares equal.
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

// recordSubmitVerify puts a submit-verify outcome somewhere durable — the
// Recorder writes through to the ndjson ledger on every Record, independent
// of how the phase ends. Nil-recorder-safe by Recorder's own contract.
func recordSubmitVerify(rec *interaction.Recorder, phase string, cycle int, site string, o submitVerifyOutcome, paste pasteOutcome) {
	payload := fmt.Sprintf("site=%s resends=%d", site, o.Resends)
	if paste.Stability != "" {
		// Paste evidence rides the success path too.
		payload += fmt.Sprintf(" paste_settle=%s stability=%s", paste.Settle, paste.Stability)
	}
	rec.Record(interaction.Outcome{
		Event: interaction.Event{
			Kind:    interaction.KindSubmitVerify,
			Phase:   phase,
			Cycle:   cycle,
			Trigger: "driver_submission",
			Payload: payload,
		},
		Result: o.Result,
	})
}

// submitVerifyBackoff is the settle after re-send n (one entry per allowed
// re-send): 500ms, 1.5s, 2.5s — the sum (4.5s) is the budget a slow TUI gets.
var submitVerifyBackoff = [submitVerifyMaxResends]time.Duration{submitVerifySettle, 3 * submitVerifySettle, 5 * submitVerifySettle}
