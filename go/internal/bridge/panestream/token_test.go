package panestream

import "testing"

// token_test.go — RED tests for cycle-428 task `unify-token-extractor`.
//
// These tests assert the contract of the unified panestream.ExtractTokenCount
// function (k-optional superset). The function does not exist yet — this file
// will not compile until Builder introduces it, making these tests RED by
// design. Do NOT modify this file to make it compile; that is Builder's job.

// TestExtractTokenCount_Unified is the primary behavioral table test.
// Covers: k-scale, plain-integer, peak-tracking, empty pane, and malformed inputs.
// Defeating a constant-return fake requires the peak + zero cases to both pass.
func TestExtractTokenCount_Unified(t *testing.T) {
	cases := []struct {
		name string
		pane string
		want int
	}{
		// k-scale forms (superset, inherited from existing stopreview contract)
		{"single integer-k", "↓ 12k tokens", 12000},
		{"single fractional-k", "↓ 5.2k tokens", 5200},
		{"half-k fractional", "↓ 3.5k tokens", 3500},
		{"quarter-k fractional", "↓ 2.5k tokens", 2500},
		// plain-integer form (new: unified superset accepts these, stopreview did not)
		{"plain integer sub-1k", "↓ 847 tokens", 847},
		{"plain integer 1", "↓ 1 tokens", 1},
		{"plain integer large", "↓ 9999 tokens", 9999},
		// peak-tracking across multiple counters
		{"multiple counters → peak", "↓ 1.2k tokens\n↓ 5.2k tokens\n↓ 3.0k tokens\n", 5200},
		{"peak is last line", "↓ 2.0k tokens\n↓ 9.9k tokens\n", 9900},
		{"plain-int peak tracking", "↓ 100 tokens\n↓ 500 tokens\n↓ 300 tokens\n", 500},
		{"mixed plain and k peak", "↓ 847 tokens\n↓ 1.2k tokens\n", 1200},
		// chrome-surrounded counter
		{"surrounded by chrome", "❯ working\n↓ 5.2k tokens · 1m 2s\nplanning\n", 5200},
		// zero / empty
		{"no counter at all", "❯ ready\nTool: Read main.go\n", 0},
		{"empty pane", "", 0},
		// malformed — must return 0 (no digits, no tokens word, non-numeric)
		{"malformed: no digits", "↓ k tokens", 0},
		{"malformed: missing tokens word", "↓ 5.2k", 0},
		{"malformed: non-numeric", "↓ abck tokens", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ExtractTokenCount(c.pane); got != c.want {
				t.Fatalf("ExtractTokenCount(%q) = %d, want %d", c.pane, got, c.want)
			}
		})
	}
}

// TestExtractTokenCount_NegativeMalformed is the adversarial negative test.
// These inputs must return 0 — a no-op implementation returning 0 would pass
// this, but would fail the positive cases above (k-scale → 12000, etc.).
func TestExtractTokenCount_NegativeMalformed(t *testing.T) {
	malformed := []string{
		"↓ k tokens",
		"↓ tokens",
		"↓  tokens",
		"no arrow here",
		"5.2k tokens without arrow",
	}
	for _, pane := range malformed {
		t.Run(pane, func(t *testing.T) {
			if got := ExtractTokenCount(pane); got != 0 {
				t.Fatalf("ExtractTokenCount(%q) = %d, want 0 (malformed input)", pane, got)
			}
		})
	}
}

// TestExtractTokenCount_PeakIsMonotonic confirms the function returns the
// maximum seen value, not the last or the first.
func TestExtractTokenCount_PeakIsMonotonic(t *testing.T) {
	pane := "↓ 500 tokens\n↓ 5.2k tokens\n↓ 300 tokens\n↓ 200 tokens\n"
	if got := ExtractTokenCount(pane); got != 5200 {
		t.Fatalf("ExtractTokenCount (peak-is-max): got %d, want 5200", got)
	}
}
