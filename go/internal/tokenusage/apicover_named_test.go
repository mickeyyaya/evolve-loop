package tokenusage

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

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

func TestResultPeakUsageNamed(t *testing.T) {
	peak := cyclestate.TokenUsage{Input: 200, CacheRead: 6_800}
	r := Result{Usage: cyclestate.TokenUsage{Input: 1_200, CacheRead: 7_800}, PeakUsage: peak}
	if r.PeakUsage != peak {
		t.Fatalf("PeakUsage = %+v, want %+v", r.PeakUsage, peak)
	}
}

func TestCollectorTypeNamed(t *testing.T) {
	var c Collector = func() Result {
		return Result{Usage: cyclestate.TokenUsage{Output: 3}, Source: SourceScrollbackPeak}
	}
	if got := Chain(c); got.Source != SourceScrollbackPeak || got.Usage.Output != 3 {
		t.Fatalf("Collector literal not run through the chain: got %+v", got)
	}
}
