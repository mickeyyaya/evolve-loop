// Package quotastate parses a CLI's captured usage/quota pane (the raw text
// returned by clicontrol.Controller.Do(family, EventUsage) through the agent
// bridge) into a STRUCTURED snapshot: per-window used fraction + reset time.
//
// This is the measurement layer of the quota-driven dynamic budgeting design:
// the existing usageprobe path already sends the usage command and classifies
// it BOOLEAN (capped?). quotastate keeps the numbers so the budget allocator
// can size the fleet wave against real headroom + time-to-reset instead of a
// static min_lanes assertion. Budgeting is in each CLI's NATIVE units
// (remaining fraction + reset), never dollars — the reason the prior
// dollar-cost budget was removed (subscription claude reports $0).
package quotastate

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Source classifies where a QuotaState came from / how trustworthy its numbers
// are. "probed" = parsed real numbers; "unknown" = no numbers parsed (CLI has
// no usage command, or the pane didn't match) → budget uses the floor fallback.
const (
	SourceProbed  = "probed"
	SourceUnknown = "unknown"
)

// Bucket is one quota window the CLI reports (e.g. session, week-all-models,
// week-per-model). UsedFraction is 0..1; RemainingFraction() = 1-UsedFraction.
type Bucket struct {
	Name         string    // normalized: "session", "week", or "week:<model>"
	Label        string    // the raw header, e.g. "Current week (all models)"
	UsedFraction float64   // 0.0–1.0, parsed from "NN% used"
	ResetAt      time.Time // parsed reset instant (zero if unparseable)
	ResetRaw     string    // the raw "Resets ..." text, kept for evidence
}

// RemainingFraction is the headroom left in this window (1 - used).
func (b Bucket) RemainingFraction() float64 { return 1 - b.UsedFraction }

// QuotaState is a family's parsed usage snapshot.
type QuotaState struct {
	Family     string
	Buckets    []Bucket
	Exhausted  bool // any bucket at ≥100% used
	Source     string
	ObservedAt time.Time
}

// TightestRemaining returns the smallest RemainingFraction across the buckets
// whose Name matches want (empty want ⇒ all buckets), and ok=false when no such
// bucket carries a number. The budget allocator sizes against the BINDING
// (tightest) window — e.g. a 34%-remaining weekly cap dominates a 73%-remaining
// session when both apply.
func (q QuotaState) TightestRemaining(want ...string) (float64, bool) {
	min, ok := 1.0, false
	for _, b := range q.Buckets {
		if len(want) > 0 && !contains(want, b.Name) {
			continue
		}
		if r := b.RemainingFraction(); r < min {
			min = r
		}
		ok = true
	}
	return min, ok
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

var (
	// "Jul 5 at 9pm" / "Jul 5 at 9:30pm" — month day at clock.
	dateAtRE = regexp.MustCompile(`(?i)^([a-z]{3,9})\s+(\d{1,2})\s+at\s+(\d{1,2})(?::(\d{2}))?\s*(am|pm)$`)
	// "4:10pm" / "9pm" — clock only (next occurrence).
	clockRE = regexp.MustCompile(`(?i)^(\d{1,2})(?::(\d{2}))?\s*(am|pm)$`)
)

var monthByPrefix = map[string]time.Month{
	"jan": time.January, "feb": time.February, "mar": time.March, "apr": time.April,
	"may": time.May, "jun": time.June, "jul": time.July, "aug": time.August,
	"sep": time.September, "oct": time.October, "nov": time.November, "dec": time.December,
}

// ParseResetWhen parses the reset formats claude's /usage emits, anchored at
// now (local location). Handles "Jul 5 at 9pm" (absolute date) and "4:10pm"
// (next occurrence of that clock time). ok=false when neither matches — the
// caller leaves ResetAt zero (a missing reset is not a fabricated one).
func ParseResetWhen(s string, now time.Time) (time.Time, bool) {
	s = strings.TrimSpace(s)
	loc := now.Location()
	if m := dateAtRE.FindStringSubmatch(s); m != nil {
		mon, ok := monthByPrefix[strings.ToLower(m[1])[:3]]
		if !ok {
			return time.Time{}, false
		}
		day, _ := strconv.Atoi(m[2])
		hour := to24h(m[3], m[5])
		minute := 0
		if m[4] != "" {
			minute, _ = strconv.Atoi(m[4])
		}
		year := now.Year()
		t := time.Date(year, mon, day, hour, minute, 0, 0, loc)
		if t.Before(now.Add(-24 * time.Hour)) { // year rollover (Dec→Jan)
			t = time.Date(year+1, mon, day, hour, minute, 0, 0, loc)
		}
		return t, true
	}
	if m := clockRE.FindStringSubmatch(s); m != nil {
		hour := to24h(m[1], m[3])
		minute := 0
		if m[2] != "" {
			minute, _ = strconv.Atoi(m[2])
		}
		t := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, loc)
		if !t.After(now) { // already passed today ⇒ next occurrence tomorrow
			t = t.Add(24 * time.Hour)
		}
		return t, true
	}
	return time.Time{}, false
}

func to24h(h, ampm string) int {
	n, _ := strconv.Atoi(h)
	pm := strings.EqualFold(ampm, "pm")
	switch {
	case pm && n != 12:
		return n + 12
	case !pm && n == 12:
		return 0
	default:
		return n
	}
}
