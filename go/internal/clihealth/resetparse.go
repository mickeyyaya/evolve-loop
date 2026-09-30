package clihealth

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParseResetHint returns when a CLI wall says its quota resets, capped at resetHintCap, or false when no hint parses.
func ParseResetHint(pane string, now time.Time) (time.Time, bool) {
	for _, hint := range resetHints {
		if at, ok := hint.parse(pane, now); ok {
			return capHint(at, now), true
		}
	}
	return time.Time{}, false
}

// resetMargin keeps the canary from probing seconds before the provider's
// clock actually rolls over.
const resetMargin = 2 * time.Minute

const resetHintCap = 24 * time.Hour

var (
	datedHintRe    = regexp.MustCompile(`(?i)try again at\s+([a-z]{3,9})\.?\s+(\d{1,2})(?:st|nd|rd|th)?,?\s+(\d{4}),?\s+(\d{1,2}):(\d{2})\s*(AM|PM)`)
	clockHintRe    = regexp.MustCompile(`(?i)(?:try again at|resets)\s+(\d{1,2}):(\d{2})\s*(AM|PM)(?:[ \t]*\(([^)]+)\))?`)
	relativeHintRe = regexp.MustCompile(`(?i)try again in\s+(?:(\d+)\s*hours?)?\s*(?:(\d+)\s*min(?:ute)?s?)?`)
)

var resetHints = []struct {
	re    *regexp.Regexp
	parse func(string, time.Time) (time.Time, bool)
}{
	{datedHintRe, parseDatedHint},
	{clockHintRe, parseClockHint},
	{relativeHintRe, parseRelativeHint},
}

func evidenceLine(pane string) string {
	for _, ln := range strings.Split(pane, "\n") {
		for _, hint := range resetHints {
			if hint.re.MatchString(ln) {
				return ln
			}
		}
	}
	return firstLine(pane)
}

func parseClockHint(pane string, now time.Time) (time.Time, bool) {
	m := clockHintRe.FindStringSubmatch(pane)
	if m == nil {
		return time.Time{}, false
	}
	// An incomplete explicit timezone must not fall back to observer-local time.
	end := clockHintRe.FindStringIndex(pane)[1]
	if strings.HasPrefix(strings.TrimLeft(pane[end:], " \t"), "(") {
		return time.Time{}, false
	}
	if m[4] != "" {
		loc, err := time.LoadLocation(m[4])
		if err != nil {
			return time.Time{}, false
		}
		now = now.In(loc)
	}
	hour, minute, ok := clockOf(m[1], m[2], m[3])
	if !ok {
		return time.Time{}, false
	}
	at := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !at.After(now) {
		// One CALENDAR day, not 24h: across a DST transition +24h lands at the
		// same wall-clock hour in the wrong offset (review finding).
		at = at.AddDate(0, 0, 1)
	}
	return at.Add(resetMargin), true
}

func parseDatedHint(pane string, now time.Time) (time.Time, bool) {
	m := datedHintRe.FindStringSubmatch(pane)
	if m == nil {
		return time.Time{}, false
	}
	month, ok := monthNamed(m[1])
	if !ok {
		return time.Time{}, false
	}
	day, _ := strconv.Atoi(m[2])
	year, _ := strconv.Atoi(m[3])
	hour, minute, ok := clockOf(m[4], m[5], m[6])
	if !ok {
		return time.Time{}, false
	}
	at := time.Date(year, month, day, hour, minute, 0, 0, now.Location())
	if at.Day() != day || !at.After(now) {
		return time.Time{}, false
	}
	return at.Add(resetMargin), true
}

func monthNamed(name string) (time.Month, bool) {
	for m := time.January; m <= time.December; m++ {
		if strings.HasPrefix(strings.ToLower(m.String()), strings.ToLower(name)) {
			return m, true
		}
	}
	return 0, false
}

func clockOf(hourText, minuteText, meridiem string) (int, int, bool) {
	hour, _ := strconv.Atoi(hourText)
	minute, _ := strconv.Atoi(minuteText)
	if hour < 1 || hour > 12 || minute > 59 {
		return 0, 0, false
	}
	if strings.EqualFold(meridiem, "PM") && hour != 12 {
		hour += 12
	}
	if strings.EqualFold(meridiem, "AM") && hour == 12 {
		hour = 0
	}
	return hour, minute, true
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
	if cap := now.Add(resetHintCap); at.After(cap) {
		return cap
	}
	return at
}
