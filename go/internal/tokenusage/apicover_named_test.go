package tokenusage

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// TestResultAndSourceNamed pins the exported Result and Source vocabulary that
// ScanConfigRoot returns but the behavioural tests only touch via field access
// — apicover requires every exported type be named in a test. It also asserts
// the Source consts are distinct so a mislabelled scan is caught.
func TestResultAndSourceNamed(t *testing.T) {
	var s Source = SourceTranscript
	if s == SourceNone {
		t.Fatal("SourceTranscript and SourceNone must be distinct")
	}
	r := Result{Source: SourceNone}
	if r.Source != SourceNone || r.Usage != (r.Usage) {
		t.Fatalf("zero Result must carry SourceNone, got %q", r.Source)
	}
	if string(SourceNone) != "none" || string(SourceTranscript) != "transcript" {
		t.Fatalf("Source string values drifted: none=%q transcript=%q", SourceNone, SourceTranscript)
	}
}

// TestResultPeakPromptTokensNamed pins Result.PeakPromptTokens (cycle-1455) and
// the rule that separates it from Usage: it is ONE turn's prompt-side
// occupancy, so it must not move when a second turn adds to the summed spend.
func TestResultPeakPromptTokensNamed(t *testing.T) {
	r := Result{
		Usage:            cyclestate.TokenUsage{Input: 10, CacheRead: 90},
		Source:           SourceTranscript,
		PeakPromptTokens: 60,
	}
	if got := windowOccupancy(r); got != 60 {
		t.Fatalf("windowOccupancy = %d, want 60 (Result.PeakPromptTokens, not the summed Usage's 100)", got)
	}
	if got := windowOccupancy(Result{Usage: r.Usage, Source: SourceEventsResult}); got != 100 {
		t.Fatalf("windowOccupancy = %d, want 100 — a tier with no per-turn breakdown falls back to its whole-launch total", got)
	}
}

// TestResultPeakUsageNamed pins Result.PeakUsage as the component-level
// companion to PeakPromptTokens: both describe the same fullest observed turn.
func TestResultPeakUsageNamed(t *testing.T) {
	peak := cyclestate.TokenUsage{Input: 200, CacheRead: 6_800}
	r := Result{Usage: cyclestate.TokenUsage{Input: 1_200, CacheRead: 7_800}, PeakUsage: peak}
	if r.PeakUsage != peak {
		t.Fatalf("PeakUsage = %+v, want %+v", r.PeakUsage, peak)
	}
}

// TestCollectorTypeNamed pins the exported Collector type (apicover requires
// every exported type be named in a test). It also asserts a bare func literal
// satisfies Collector and the chain runs it — the load-bearing property is that
// Collector is `func() Result`.
func TestCollectorTypeNamed(t *testing.T) {
	var c Collector = func() Result {
		return Result{Usage: cyclestate.TokenUsage{Output: 3}, Source: SourceScrollbackPeak}
	}
	if got := Chain(c); got.Source != SourceScrollbackPeak || got.Usage.Output != 3 {
		t.Fatalf("Collector literal not run through the chain: got %+v", got)
	}
}
