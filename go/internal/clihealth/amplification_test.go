package clihealth

import (
	"testing"
	"time"
	"unicode/utf8"
)

func TestAmplifiedBenchableRejectsPatternNameVariants(t *testing.T) {
	if !Benchable("rate_limit") {
		t.Fatalf("Benchable(rate_limit) = false, want true")
	}

	for _, pattern := range []string{" rate_limit", "rate_limit ", "RATE_LIMIT", "rate-limit", "auth_recheck"} {
		if Benchable(pattern) {
			t.Fatalf("Benchable(%q) = true, want false for exact closed-set matching", pattern)
		}
	}
}

func TestAmplifiedNewBenchEntryParsesHintAfterEmptyEvidenceLine(t *testing.T) {
	now := time.Date(2026, 6, 13, 9, 30, 0, 0, time.UTC)
	prev := Entry{Family: "codex", Reason: "rate_limit", Strikes: 2}
	paneText := "\ntry again at 6:11 AM"

	got := NewBenchEntry(prev, "codex", "rate_limit", paneText, now)

	if got.Family != "codex" || got.Reason != "rate_limit" {
		t.Fatalf("identity fields = (%q, %q), want (codex, rate_limit)", got.Family, got.Reason)
	}
	if got.Strikes != 3 {
		t.Fatalf("Strikes = %d, want 3", got.Strikes)
	}
	if got.BenchedAt != now {
		t.Fatalf("BenchedAt = %s, want %s", got.BenchedAt, now)
	}
	// Evidence is now the wall BANNER line (the reset-hint line), not the empty
	// first line — evidenceLine picks the line that actually walled the CLI
	// (bridge-ratelimit evidence-accuracy fix), strictly more useful than a
	// leading blank line for forensics.
	if got.Evidence != "try again at 6:11 AM" {
		t.Fatalf("Evidence = %q, want the reset-hint banner line", got.Evidence)
	}
	wantUntil, ok := ParseResetHint(paneText, now)
	if !ok {
		t.Fatalf("ParseResetHint(%q) failed, test fixture is invalid", paneText)
	}
	if !got.BenchedUntil.Equal(wantUntil) {
		t.Fatalf("BenchedUntil = %s, want parsed reset hint %s", got.BenchedUntil, wantUntil)
	}
}

func TestAmplifiedTruncateRunesKeepsUTF8ValidAtTightLimits(t *testing.T) {
	input := "quota exhausted: 日本語🚀"

	for _, limit := range []int{1, 2, 3, 4, 5} {
		got := truncateRunes(input, limit)
		if !utf8.ValidString(got) {
			t.Fatalf("truncateRunes(%q, %d) returned invalid UTF-8: %q", input, limit, got)
		}
		if len([]rune(got)) > limit {
			t.Fatalf("truncateRunes(%q, %d) returned %d runes, want <= limit: %q", input, limit, len([]rune(got)), got)
		}
	}
}
