package clicontrol

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// This file adds the READING half of the usage control.
//
// Resolving WHICH command to send was already abstracted: the caller names a
// family and EventUsage, and the Controller looks up that family's own
// spelling in its manifest `controls` table (/status for codex, /usage for
// claude and agy, absent for ollama). But the reply was returned as a raw
// pane, and its only consumer asked a yes/no question — "does this look
// exhausted?" — so nothing could answer "how much is left?".
//
// Each CLI answers in its own format and even in its own DIRECTION (codex
// reports remaining, claude reports consumed), so each family gets its own
// parser here and the caller sees neither. Adding a CLI means adding a parser
// plus a manifest entry; it never means touching a caller.

// Window is one rate-limit window a CLI reports (weekly, 5h, …).
type Window struct {
	// Name is the CLI's own label for the window, kept verbatim so an
	// operator reading a report sees what the CLI actually said.
	Name string
	// PercentLeft is REMAINING headroom, 0..100 — normalized across CLIs that
	// report remaining and CLIs that report consumed.
	PercentLeft int
	// ResetHint is the CLI's own reset wording, kept as text rather than
	// parsed into a time: formats and timezones vary per CLI and per locale,
	// and a mis-parsed timestamp is more dangerous than an unparsed string.
	ResetHint string
}

// Usage is a normalized reading of one family's quota standing.
type Usage struct {
	Family string
	// Windows is empty when Parsed is false.
	Windows []Window
	// Parsed distinguishes "we read the figures" from "we captured a pane we
	// could not read". Both are legitimate outcomes; conflating them is how a
	// reader ends up reporting a confident number for output it never
	// understood.
	Parsed bool
	// Raw is the captured pane, always retained — on a parse miss it is the
	// only evidence of what the CLI actually displayed.
	Raw string
}

// MinPercentLeft returns the tightest window's remaining headroom, or -1 when
// nothing was parsed.
//
// -1 rather than 0 is deliberate: 0 means "exhausted", and a caller throttling
// on an unreadable pane would stop work that could have run. Unknown must be
// distinguishable from empty.
func (u Usage) MinPercentLeft() int {
	if !u.Parsed || len(u.Windows) == 0 {
		return -1
	}
	min := u.Windows[0].PercentLeft
	for _, w := range u.Windows[1:] {
		if w.PercentLeft < min {
			min = w.PercentLeft
		}
	}
	return min
}

// UsageParser turns one family's own pane text into windows. ok is false when
// the pane carries no figures this parser recognises.
type UsageParser func(pane string) (windows []Window, ok bool)

// usageParsers is the per-family Strategy registry — the reading counterpart
// of the manifest `controls` table that resolves the command. A family absent
// here parses to "unknown", never to zero.
//
// agy is deliberately ABSENT: its /usage output has not been captured from a
// live CLI, and writing a parser against a guessed format would produce
// confident numbers for a pane nobody has seen. Add it with a verbatim fixture
// when one exists.
var usageParsers = map[string]UsageParser{
	"codex":  parseCodexUsage,
	"claude": parseClaudeUsage,
}

// codexUsageLine matches codex's `/status` rows, e.g.
//
//	Weekly limit:   [████████████████████] 100% left (resets 00:45 on 5 Sep)
//
// The percentage is REQUIRED by the pattern, which is what keeps codex's
// bodiless "GPT-5.3-Codex-Spark limit:" heading from becoming a phantom
// window reporting 0% left.
var codexUsageLine = regexp.MustCompile(`([A-Za-z0-9.\- ]*?limit):\s*(?:\[[^\]]*\]\s*)?(\d{1,3})%\s*left\s*(\([^)]*\))?`)

func parseCodexUsage(pane string) ([]Window, bool) {
	var out []Window
	for _, m := range codexUsageLine.FindAllStringSubmatch(pane, -1) {
		pct, err := strconv.Atoi(m[2])
		if err != nil || pct < 0 || pct > 100 {
			continue
		}
		out = append(out, Window{
			Name:        strings.TrimSpace(m[1]),
			PercentLeft: pct,
			ResetHint:   strings.Trim(strings.TrimSpace(m[3]), "()"),
		})
	}
	return out, len(out) > 0
}

// claudeUsageLine matches claude's own phrasing, e.g.
//
//	You've used 97% of your weekly limit · resets Aug 30 at 9pm (Asia/Taipei)
//
// claude states what has been CONSUMED where codex states what REMAINS, so
// this parser inverts. Reporting 97 as headroom when 97 was spent is the most
// dangerous direction this code could get wrong, which is why the test names
// that case explicitly.
var claudeUsageLine = regexp.MustCompile(`used\s+(\d{1,3})%\s+of\s+your\s+(\w+)\s+limit(?:\s*[·|-]\s*(resets[^\n]*))?`)

func parseClaudeUsage(pane string) ([]Window, bool) {
	m := claudeUsageLine.FindStringSubmatch(pane)
	if m == nil {
		return nil, false
	}
	used, err := strconv.Atoi(m[1])
	if err != nil || used < 0 || used > 100 {
		return nil, false
	}
	return []Window{{
		Name:        m[2] + " limit",
		PercentLeft: 100 - used,
		ResetHint:   strings.TrimSpace(m[3]),
	}}, true
}

// ParseUsage reads a family's usage pane into normalized form. It never
// fabricates: an unrecognised pane, or a family with no parser, yields
// Parsed=false with the raw text retained.
func ParseUsage(family, pane string) (Usage, bool) {
	u := Usage{Family: family, Raw: pane}
	parse, ok := usageParsers[family]
	if !ok {
		return u, false
	}
	windows, ok := parse(pane)
	if !ok {
		return u, false
	}
	u.Windows, u.Parsed = windows, true
	return u, true
}

// QueryUsage asks one family how much quota it has left, through the
// abstraction: it names EventUsage, never a command. Which command that
// becomes — codex's /status, claude's or agy's /usage — is resolved by the
// Controller from that family's manifest.
//
// Three outcomes, deliberately distinct:
//   - error wrapping ErrUnsupported — the family has no usage surface at all
//     (ollama). Branch with errors.Is.
//   - error otherwise — the control itself failed (REPL down, timeout).
//   - nil error with Parsed=false — the control ran and we captured a pane we
//     could not read. Not a failure, and not a zero.
func QueryUsage(ctx context.Context, ctrl Controller, family string) (Usage, error) {
	if ctrl == nil {
		return Usage{Family: family}, fmt.Errorf("clicontrol: QueryUsage requires a Controller")
	}
	resp, err := ctrl.Do(ctx, family, EventUsage)
	if err != nil {
		// Returned unwrapped-in-spirit: ErrUnsupported must stay matchable by
		// errors.Is so a caller can tell "no such surface" from "it broke".
		return Usage{Family: family}, err
	}
	u, _ := ParseUsage(family, resp.Pane)
	return u, nil
}
