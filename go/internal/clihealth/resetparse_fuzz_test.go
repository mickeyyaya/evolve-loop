package clihealth

// Fuzz harness for ParseResetHint (inbox fuzz-parser-surfaces, slice 1). Seeded
// from the TestParseResetHintTable goldens (cycle-283/cycle-314 wall shapes) so
// the corpus starts from real provider wording, then explores mutations for
// crashers the curated table can't anticipate. `now` is held fixed (fuzz
// corpora can't carry a time.Time) so every invariant check is deterministic.
//
// Invariants asserted on every input (never just "no panic"):
//   - ok=false must return the zero time.Time (no half-parsed leakage).
//   - ok=true must return a time strictly after now (no negative "duration
//     until reset") and no more than 24h after now (the capHint ceiling).

import (
	"testing"
	"time"
)

func FuzzParseResetHint(f *testing.F) {
	for _, seed := range []string{
		"or try again at 6:11 AM.",
		"try again at 5:26 PM.",
		"try again at 12:10 AM",
		"try again at 12:00 PM",
		"try again at 12:00 AM",
		"TRY AGAIN AT 6:11 am",
		"Rate limited. Try again in 2 hours.",
		"try again in 45 minutes",
		"try again in 5 mins",
		"try again in 1 hour 30 minutes",
		"try again in 1 hour",
		"try again in 90 hours",
		"no hint here",
		"",
		"try again at 13:00 PM",
		"try again at 6:75 AM",
		"please try again later",
		"try again at 0:00 AM",
		"try again in 0 minutes",
		"try again in -5 minutes",
		"try again at 6:11 AM\x00\xff",
		"try again in 999999999999999999999 hours",
	} {
		f.Add(seed)
	}

	now := time.Date(2026, 6, 11, 0, 30, 0, 0, time.FixedZone("TEST", 8*3600))

	f.Fuzz(func(t *testing.T, pane string) {
		got, ok := ParseResetHint(pane, now)
		if !ok {
			if !got.IsZero() {
				t.Fatalf("ParseResetHint(%q) ok=false but returned non-zero time %v", pane, got)
			}
			return
		}
		if got.Before(now) {
			t.Fatalf("ParseResetHint(%q) returned %v, which is BEFORE now=%v (negative duration until reset)", pane, got, now)
		}
		if max := now.Add(24 * time.Hour); got.After(max) {
			t.Fatalf("ParseResetHint(%q) returned %v, more than 24h after now=%v (far-future reset escaped the cap)", pane, got, now)
		}
	})
}
