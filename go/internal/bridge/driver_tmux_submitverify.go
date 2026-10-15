package bridge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

const (
	submitVerifyMaxResends   = 3
	submitVerifyEchoRunes    = 40
	submitVerifyMinEchoRunes = 8
	submitVerifySettle       = 500 * time.Millisecond
	tmuxPastePlaceholderEcho = "[Pasted text"
)

func normalizeWS(s string) string { return strings.Join(strings.Fields(s), " ") }

func echoChunk(s string) string {
	r := []rune(normalizeWS(s))
	if len(r) > submitVerifyEchoRunes {
		r = r[:submitVerifyEchoRunes]
	}
	return string(r)
}

func pendingAtInputLine(pane, marker string, echoes []string) bool {
	if marker == "" {
		return false
	}
	rest, found := afterLastInputLine(pane, marker)
	if !found {
		return false
	}
	tail := normalizeWS(rest)
	if tail == "" {
		return false
	}
	for _, e := range echoes {
		full := normalizeWS(e)
		if full == "" {
			continue
		}
		if chunk := echoChunk(full); len([]rune(chunk)) >= submitVerifyMinEchoRunes && strings.Contains(tail, chunk) {
			return true
		}
		if len([]rune(tail)) >= submitVerifyMinEchoRunes && strings.Contains(full, tail) {
			return true
		}
	}
	return false
}

type submitVerifyOutcome struct {
	Resends int
	Result  string
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

func afterLastInputLine(pane, marker string) (string, bool) {
	lines := strings.Split(pane, "\n")
	i := panestream.LastMarkerLine(lines, marker)
	if i < 0 {
		return "", false
	}
	box := strings.TrimPrefix(strings.TrimLeft(lines[i], " \t"), marker)
	return strings.Join(append([]string{box}, lines[i+1:]...), "\n"), true
}

func promptSubmitEcho(prompt string) string { return echoChunk(prompt) }

func firstNonEmptyLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		if strings.TrimSpace(ln) != "" {
			return ln
		}
	}
	return ""
}

func recordSubmitVerify(rec *interaction.Recorder, phase string, cycle int, site string, o submitVerifyOutcome, paste pasteOutcome) {
	payload := fmt.Sprintf("site=%s resends=%d", site, o.Resends)
	if paste.Stability != "" {
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

var submitVerifyBackoff = [submitVerifyMaxResends]time.Duration{submitVerifySettle, 3 * submitVerifySettle, 5 * submitVerifySettle}
