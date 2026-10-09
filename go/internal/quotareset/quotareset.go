// Package quotareset computes the wake-up time of a quota pause from the best evidence it has.
// See docs/architecture/packages/internal-quotareset.md.
package quotareset

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var hintTimeRE = regexp.MustCompile(`(?i)(\d{1,2}):(\d{2})(am|pm)`)

type Result struct {
	WakeAt time.Time
	ISO    string
	Source string
}

// Options exposes seams for testing.
type Options struct {
	Now          func() time.Time
	ResetAt      string
	DefaultHours float64
	BenchedUntil time.Time
	UsageReset   func() (time.Time, bool)
}

func Compute(workspace string, opts Options) (Result, error) {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	if override := strings.TrimSpace(opts.ResetAt); override != "" {
		return overrideResult(override, now()), nil
	}
	if t, ok := hintReset(workspace, now()); ok {
		return resultAt(t, "parsed"), nil
	}
	if opts.BenchedUntil.After(now()) {
		return resultAt(opts.BenchedUntil, "bench"), nil
	}
	if t, ok := usageReset(opts.UsageReset, now()); ok {
		return resultAt(t, "usage"), nil
	}
	if opts.DefaultHours > 0 {
		return resultAt(now().Add(time.Duration(opts.DefaultHours*float64(time.Hour))), "default"), nil
	}
	return resultAt(now(), "unknown"), nil
}

func usageReset(query func() (time.Time, bool), now time.Time) (time.Time, bool) {
	if query == nil {
		return time.Time{}, false
	}
	t, ok := query()
	return t, ok && t.After(now)
}

func overrideResult(override string, now time.Time) Result {
	t, err := time.Parse(time.RFC3339, override)
	if err != nil {
		return Result{WakeAt: now, ISO: override, Source: "operator-override"}
	}
	return Result{WakeAt: t, ISO: override, Source: "operator-override"}
}

func hintReset(workspace string, now time.Time) (time.Time, bool) {
	if workspace == "" {
		return time.Time{}, false
	}
	raw, err := os.ReadFile(filepath.Join(workspace, "quota-reset-hint.txt"))
	if err != nil {
		return time.Time{}, false
	}
	hint := strings.TrimSpace(string(raw))
	if len(hint) > 32 {
		hint = hint[:32]
	}
	return parseHint(hint, now)
}

func resultAt(t time.Time, source string) Result {
	return Result{WakeAt: t, ISO: isoFormat(t), Source: source}
}

// parseHint extracts a "HH:MM(am|pm)" time and returns the next
// occurrence (today if still future, else tomorrow).
func parseHint(hint string, nowT time.Time) (time.Time, bool) {
	// Strip whitespace
	hint = strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, hint)
	m := hintTimeRE.FindStringSubmatch(hint)
	if len(m) != 4 {
		return time.Time{}, false
	}
	hh, err := strconv.Atoi(m[1])
	if err != nil {
		return time.Time{}, false
	}
	mm, err := strconv.Atoi(m[2])
	if err != nil {
		return time.Time{}, false
	}
	ampm := strings.ToLower(m[3])
	switch ampm {
	case "pm":
		if hh < 12 {
			hh += 12
		}
	case "am":
		if hh == 12 {
			hh = 0
		}
	}
	if hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return time.Time{}, false
	}
	candidate := time.Date(nowT.Year(), nowT.Month(), nowT.Day(), hh, mm, 0, 0, nowT.Location())
	if !candidate.After(nowT) {
		candidate = candidate.Add(24 * time.Hour)
	}
	return candidate, true
}

// isoFormat returns the canonical ISO 8601 string with local-TZ offset
// matching the bash output format "%Y-%m-%dT%H:%M:%S%z".
func isoFormat(t time.Time) string {
	return t.Format("2006-01-02T15:04:05-0700")
}

// Format returns the 2-line stdout shape produced by the bash script.
func (r Result) Format() string {
	return fmt.Sprintf("%s\nsource=%s\n", r.ISO, r.Source)
}
