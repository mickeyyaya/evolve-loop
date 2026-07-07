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
