package clihealth

// Slice-2 hardening table for ParseResetHint. The clock-time grammar is the
// cycle-283 codex wall shape; the relative grammar covers the other wording
// providers use. Everything time-sensitive injects now — no wall-clock reads.

import (
	"testing"
	"time"
)

func TestParseResetHintTable(t *testing.T) {
	t.Parallel()
	loc := time.FixedZone("TEST", 8*3600) // deterministic non-UTC zone
	now := time.Date(2026, 6, 11, 0, 30, 0, 0, loc)
	day := 24 * time.Hour

	cases := []struct {
		name string
		pane string
		want time.Time
		ok   bool
	}{
		{"cycle283 AM ahead", "or try again at 6:11 AM.", time.Date(2026, 6, 11, 6, 13, 0, 0, loc), true},
		{"PM same day", "try again at 5:26 PM.", time.Date(2026, 6, 11, 17, 28, 0, 0, loc), true},
		{"passed today rolls to tomorrow", "try again at 12:10 AM", time.Date(2026, 6, 12, 0, 12, 0, 0, loc), true},
		{"noon", "try again at 12:00 PM", time.Date(2026, 6, 11, 12, 2, 0, 0, loc), true},
		{"midnight as 12 AM tomorrow", "try again at 12:00 AM", time.Date(2026, 6, 12, 0, 2, 0, 0, loc), true},
		{"case-insensitive", "TRY AGAIN AT 6:11 am", time.Date(2026, 6, 11, 6, 13, 0, 0, loc), true},
		{"relative hours", "Rate limited. Try again in 2 hours.", now.Add(2*time.Hour + resetMargin), true},
		{"relative minutes", "try again in 45 minutes", now.Add(45*time.Minute + resetMargin), true},
		{"relative mins short", "try again in 5 mins", now.Add(5*time.Minute + resetMargin), true},
		{"relative combined", "try again in 1 hour 30 minutes", now.Add(90*time.Minute + resetMargin), true},
		{"relative singular", "try again in 1 hour", now.Add(time.Hour + resetMargin), true},
		{"capped at 24h", "try again in 90 hours", now.Add(day), true},
		{"garbage", "no hint here", time.Time{}, false},
		{"empty", "", time.Time{}, false},
		{"invalid hour", "try again at 13:00 PM", time.Time{}, false},
		{"invalid minute", "try again at 6:75 AM", time.Time{}, false},
		{"bare try again", "please try again later", time.Time{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ParseResetHint(c.pane, now)
			if ok != c.ok {
				t.Fatalf("ok=%v, want %v (got=%v)", ok, c.ok, got)
			}
			if ok && !got.Equal(c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

// TestParseResetHintHonorsNowLocation: the printed clock time is interpreted
// in now's location (CLIs print host-local times).
func TestParseResetHintHonorsNowLocation(t *testing.T) {
	t.Parallel()
	tokyo := time.FixedZone("UTC+9", 9*3600)
	now := time.Date(2026, 6, 11, 1, 0, 0, 0, tokyo)
	got, ok := ParseResetHint("try again at 6:11 AM", now)
	if !ok {
		t.Fatal("must parse")
	}
	if got.Location() != tokyo {
		t.Errorf("result location %v, want now's location", got.Location())
	}
	want := time.Date(2026, 6, 11, 6, 13, 0, 0, tokyo)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestEvidenceLine_PrefersBannerOverFirstLine pins the bench-evidence accuracy
// fix (bridge-ratelimit, cycle-314 forensics): the bench Evidence must be the
// actual wall BANNER line (the one carrying the reset hint), not the pane's
// first line — which on a scrolled pane caught a later frame ("53 +\tFamily:
// codex,", an agent edit-diff line) and obscured what really walled the CLI.
func TestEvidenceLine_PrefersBannerOverFirstLine(t *testing.T) {
	pane := "   53 +\t\tFamily: \"codex\",\n" +
		"some intermediate output\n" +
		"You've hit your usage limit. try again in 3 hours.\n"
	got := evidenceLine(pane)
	if got != "You've hit your usage limit. try again in 3 hours." {
		t.Errorf("evidenceLine picked %q, want the reset-hint banner line", got)
	}
}

// TestEvidenceLine_FallsBackToFirstLine: with no reset-hint banner present,
// keep the legacy firstLine behavior (no regression).
func TestEvidenceLine_FallsBackToFirstLine(t *testing.T) {
	pane := "first line here\nsecond line\n"
	if got := evidenceLine(pane); got != "first line here" {
		t.Errorf("evidenceLine = %q, want firstLine fallback", got)
	}
}
