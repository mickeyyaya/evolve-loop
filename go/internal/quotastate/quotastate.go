// Package quotastate parses a CLI's captured usage pane into a structured quota snapshot.
// Each window reports its used fraction and reset time, in the CLI's native units.
package quotastate

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	SourceProbed  = "probed"
	SourceUnknown = "unknown"
)

type Bucket struct {
	Name         string
	Label        string
	UsedFraction float64
	ResetAt      time.Time
	ResetRaw     string
}

func (b Bucket) RemainingFraction() float64 { return 1 - b.UsedFraction }

type QuotaState struct {
	Family     string
	Buckets    []Bucket
	Exhausted  bool
	Source     string
	ObservedAt time.Time
}

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
	monthDayAtClockRE = regexp.MustCompile(`(?i)^([a-z]{3,9})\s+(\d{1,2})\s+at\s+(\d{1,2})(?::(\d{2}))?\s*(am|pm)$`)
	clockOnlyRE       = regexp.MustCompile(`(?i)^(\d{1,2})(?::(\d{2}))?\s*(am|pm)$`)
)

var monthByPrefix = map[string]time.Month{
	"jan": time.January, "feb": time.February, "mar": time.March, "apr": time.April,
	"may": time.May, "jun": time.June, "jul": time.July, "aug": time.August,
	"sep": time.September, "oct": time.October, "nov": time.November, "dec": time.December,
}

func ParseResetWhen(s string, now time.Time) (time.Time, bool) {
	s = strings.TrimSpace(s)
	loc := now.Location()
	if m := monthDayAtClockRE.FindStringSubmatch(s); m != nil {
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
		wentPastInPriorYear := t.Before(now.Add(-24 * time.Hour))
		if wentPastInPriorYear {
			t = time.Date(year+1, mon, day, hour, minute, 0, 0, loc)
		}
		return t, true
	}
	if m := clockOnlyRE.FindStringSubmatch(s); m != nil {
		hour := to24h(m[1], m[3])
		minute := 0
		if m[2] != "" {
			minute, _ = strconv.Atoi(m[2])
		}
		t := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, loc)
		alreadyPassedToday := !t.After(now)
		if alreadyPassedToday {
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
