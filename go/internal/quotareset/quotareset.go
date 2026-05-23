// Package quotareset ports legacy/scripts/dispatch/estimate-quota-reset.sh.
//
// Computes wake-up time after a Claude Code quota hit. Source priority:
//  1. EVOLVE_QUOTA_RESET_AT env — operator-supplied ISO 8601 override
//  2. Hint file at $WORKSPACE/quota-reset-hint.txt — Anthropic's
//     "resets HH:MMam" message captured by claude.sh stderr filter
//  3. Fallback: now + EVOLVE_QUOTA_RESET_HOURS (default 5h25min)
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

// Result captures the computed wake-up time + the source that produced it.
type Result struct {
	WakeAt time.Time
	ISO    string // RFC3339-format ISO 8601 in local TZ
	Source string // "operator-override", "parsed", or "default"
}

// Options exposes seams for testing.
type Options struct {
	Env     func(name string) string // defaults to os.Getenv
	Now     func() time.Time         // defaults to time.Now
	HoursFn func() float64           // defaults to reading EVOLVE_QUOTA_RESET_HOURS
}

// Compute runs the source-priority chain and returns a Result.
// workspace may be empty (skips source 2). Returns error only when
// all three sources fail (extremely rare — would mean malformed
// env override AND missing/malformed hint file AND a bad hours env).
func Compute(workspace string, opts Options) (Result, error) {
	getEnv := opts.Env
	if getEnv == nil {
		getEnv = os.Getenv
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	// Source 1: operator override
	if override := strings.TrimSpace(getEnv("EVOLVE_QUOTA_RESET_AT")); override != "" {
		// Parse to populate WakeAt — accept the operator's string verbatim
		// as the canonical ISO if it parses; otherwise use it raw.
		t, err := time.Parse(time.RFC3339, override)
		if err != nil {
			// keep override string but synthesize WakeAt = now
			return Result{WakeAt: now(), ISO: override, Source: "operator-override"}, nil
		}
		return Result{WakeAt: t, ISO: override, Source: "operator-override"}, nil
	}

	// Source 2: hint file
	if workspace != "" {
		hintPath := filepath.Join(workspace, "quota-reset-hint.txt")
		if info, err := os.Stat(hintPath); err == nil && info.Size() > 0 {
			raw, err := os.ReadFile(hintPath)
			if err == nil {
				hint := strings.TrimSpace(string(raw))
				if len(hint) > 32 {
					hint = hint[:32]
				}
				if t, ok := parseHint(hint, now()); ok {
					return Result{WakeAt: t, ISO: isoFormat(t), Source: "parsed"}, nil
				}
			}
		}
	}

	// Source 3: fallback hours
	hours := 5.4167
	if opts.HoursFn != nil {
		hours = opts.HoursFn()
	} else if h := getEnv("EVOLVE_QUOTA_RESET_HOURS"); h != "" {
		if v, err := strconv.ParseFloat(h, 64); err == nil {
			hours = v
		}
	}
	wake := now().Add(time.Duration(hours * float64(time.Hour)))
	return Result{WakeAt: wake, ISO: isoFormat(wake), Source: "default"}, nil
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
	re := regexp.MustCompile(`(?i)(\d{1,2}):(\d{2})(am|pm)`)
	m := re.FindStringSubmatch(hint)
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
