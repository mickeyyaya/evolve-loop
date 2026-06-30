package bridge

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

// tokenextractor_unified_test.go — Cross-package RED test for cycle-428
// task `unify-token-extractor` (AC3).
//
// This file asserts that the BRIDGE CONSUMER PATH uses panestream.ExtractTokenCount
// — the exported unified extractor — for both plain-integer and k-scale forms.
// The function does not exist yet; this file fails to compile until Builder
// introduces it. Do NOT modify to pass; that is Builder's job.
//
// AC3 cross-package proof: if panestream.ExtractTokenCount exists and accepts
// both forms, the bridge package can import and call it directly. This
// demonstrates the projection seam (ADR-0047) is wired correctly.

// TestUnifiedTokenExtractor_PlainInt asserts the plain "↓ N tokens" form
// (no k suffix) returns N via panestream.ExtractTokenCount.
// This is the key behavioral change from stopreview's strict parser (which
// returned 0 for plain integers — the under-counting bug).
func TestUnifiedTokenExtractor_PlainInt(t *testing.T) {
	cases := []struct {
		name string
		pane string
		want int
	}{
		{"sub-1k plain", "↓ 847 tokens", 847},
		{"unit plain", "↓ 1 tokens", 1},
		{"near-1k plain", "↓ 999 tokens", 999},
		{"plain in chrome context", "❯ working\n↓ 847 tokens · 0m 5s\n", 847},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := panestream.ExtractTokenCount(c.pane)
			if got != c.want {
				t.Fatalf("panestream.ExtractTokenCount(%q) = %d, want %d (plain-int form)", c.pane, got, c.want)
			}
		})
	}
}

// TestUnifiedTokenExtractor_KScale asserts the k-scale "↓ N.Nk tokens" form
// still works via panestream.ExtractTokenCount (regression guard).
func TestUnifiedTokenExtractor_KScale(t *testing.T) {
	cases := []struct {
		name string
		pane string
		want int
	}{
		{"fractional k", "↓ 5.2k tokens", 5200},
		{"integer k", "↓ 12k tokens", 12000},
		{"half k", "↓ 3.5k tokens", 3500},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := panestream.ExtractTokenCount(c.pane)
			if got != c.want {
				t.Fatalf("panestream.ExtractTokenCount(%q) = %d, want %d (k-scale form)", c.pane, got, c.want)
			}
		})
	}
}

// TestUnifiedTokenExtractor_EmptyAndMalformed is the adversarial negative test:
// empty and malformed inputs must return 0 via the exported function.
func TestUnifiedTokenExtractor_EmptyAndMalformed(t *testing.T) {
	cases := []struct {
		name string
		pane string
	}{
		{"empty", ""},
		{"no counter", "❯ ready\nTool: Read main.go\n"},
		{"malformed no digits", "↓ k tokens"},
		{"malformed missing tokens word", "↓ 5.2k"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := panestream.ExtractTokenCount(c.pane)
			if got != 0 {
				t.Fatalf("panestream.ExtractTokenCount(%q) = %d, want 0 (malformed)", c.pane, got)
			}
		})
	}
}
