package clihealth

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParseResetHint extracts a benched-until time from CLI wall text. Two hint
// shapes are recognized (case-insensitive):
//
//	"try again at 6:11 AM"   → the NEXT occurrence of that local clock time
//	"try again in 2 hours" / "in 45 minutes" / "in 1 hour 30 minutes"
//
// A small safety margin is added so the canary fires after the quota actually
// resets, and the result is capped at now+24h (a hint further out than a day
// is more likely a parse artifact than a real reset). Returns ok=false when
// no hint parses — the caller falls back to CooldownForStrikes.
//
// The clock time is interpreted in now's location: CLIs print wall text in
// the host's local timezone (verified against the cycle-283 codex transcript).
func ParseResetHint(pane string, now time.Time) (time.Time, bool) {
	if at, ok := parseClockHint(pane, now); ok {
		return capHint(at, now), true
	}
	if at, ok := parseRelativeHint(pane, now); ok {
		return capHint(at, now), true
	}
	return time.Time{}, false
}

// resetMargin keeps the canary from probing seconds before the provider's
// clock actually rolls over.
const resetMargin = 2 * time.Minute

var (
	clockHintRe    = regexp.MustCompile(`(?i)try again at\s+(\d{1,2}):(\d{2})\s*(AM|PM)`)
	relativeHintRe = regexp.MustCompile(`(?i)try again in\s+(?:(\d+)\s*hours?)?\s*(?:(\d+)\s*min(?:ute)?s?)?`)
)

// evidenceLine returns the most representative line for a bench record: the
// wall BANNER line that carries the reset hint (the line ParseResetHint keys
// on), which is what actually walled the CLI — not the pane's first line. On a
// scrolled pane firstLine catches a later frame or, as in cycle-314, the
// agent's own edit content ("53 +\tFamily: codex,"), obscuring the real cause
// in the bench evidence. Falls back to firstLine when no reset-hint line is
// present (no regression for walls without a parseable hint).
func evidenceLine(pane string) string {
	for _, ln := range strings.Split(pane, "\n") {
		if clockHintRe.MatchString(ln) || relativeHintRe.MatchString(ln) {
			return ln
		}
	}
	return firstLine(pane)
}

func parseClockHint(pane string, now time.Time) (time.Time, bool) {
	m := clockHintRe.FindStringSubmatch(pane)
	if m == nil {
		return time.Time{}, false
	}
	hour, _ := strconv.Atoi(m[1])
	minute, _ := strconv.Atoi(m[2])
	if hour < 1 || hour > 12 || minute > 59 {
		return time.Time{}, false
	}
	if strings.EqualFold(m[3], "PM") && hour != 12 {
		hour += 12
	}
	if strings.EqualFold(m[3], "AM") && hour == 12 {
		hour = 0
	}
	at := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !at.After(now) {
		// One CALENDAR day, not 24h: across a DST transition +24h lands at the
		// same wall-clock hour in the wrong offset (review finding).
		at = at.AddDate(0, 0, 1)
	}
	return at.Add(resetMargin), true
}

func parseRelativeHint(pane string, now time.Time) (time.Time, bool) {
	m := relativeHintRe.FindStringSubmatch(pane)
	if m == nil || (m[1] == "" && m[2] == "") {
		return time.Time{}, false
	}
	var d time.Duration
	if m[1] != "" {
		h, _ := strconv.Atoi(m[1])
		d += time.Duration(h) * time.Hour
	}
	if m[2] != "" {
		mins, _ := strconv.Atoi(m[2])
		d += time.Duration(mins) * time.Minute
	}
	if d <= 0 {
		return time.Time{}, false
	}
	return now.Add(d).Add(resetMargin), true
}

func capHint(at, now time.Time) time.Time {
	if cap := now.Add(24 * time.Hour); at.After(cap) {
		return cap
	}
	return at
}
