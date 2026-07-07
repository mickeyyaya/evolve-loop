package tokenusage

// chain.go — token-telemetry S2 collector chain (docs/plans/
// token-telemetry-2026-07.md S2). A cycle's token usage can be recovered from
// several sources of differing fidelity; the chain composes them in fidelity
// order — transcript > eventsResult > scrollbackPeak — and returns the first
// NON-empty tier, recording which source produced the figure. Each tier is a
// lazily-evaluated Collector so a higher tier that succeeds spares the cost of
// running the lower ones.

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclecost"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

const (
	// SourceEventsResult means usage was recovered from a phase's
	// *-events.ndjson result envelope (the same source cyclecost reads).
	SourceEventsResult Source = "events_result"
	// SourceScrollbackPeak means usage was recovered from the pane scrollback's
	// peak ↓ response-token marker — an output-only floor, the lowest fidelity.
	SourceScrollbackPeak Source = "scrollback_peak"
)

// Collector is a lazily-evaluated usage source. It returns the zero Result
// (Source == SourceNone) when it has no data; the chain treats SourceNone as
// "empty" and falls through to the next tier.
type Collector func() Result

// Chain runs collectors in the given (fidelity) order and returns the first
// Result whose Source != SourceNone. An all-empty chain yields SourceNone with
// zero usage — it never spuriously returns the first tier.
func Chain(collectors ...Collector) Result {
	for _, c := range collectors {
		if r := c(); r.Source != SourceNone {
			return r
		}
	}
	return Result{Source: SourceNone}
}

// TranscriptCollector wraps ScanConfigRoot as the highest-fidelity tier. A root
// with no matching transcript (or a scan error — telemetry is best-effort)
// reports SourceNone so the chain falls through.
func TranscriptCollector(root string, w Window) Collector {
	return func() Result {
		r, err := ScanConfigRoot(root, w)
		if err != nil || r.Source == SourceNone {
			return Result{Source: SourceNone}
		}
		return r
	}
}

// EventsResultCollector reuses cyclecost.ParseEventsLog — the single canonical
// result-envelope extractor — so the recovered counts match cyclecost by
// construction (no duplicated parser). An envelope-less log yields SourceNone.
func EventsResultCollector(logPath string) Collector {
	return func() Result {
		pc, ok := cyclecost.ParseEventsLog(logPath)
		if !ok {
			return Result{Source: SourceNone}
		}
		return Result{
			Usage: cyclestate.TokenUsage{
				Input:      int(pc.InputTokens),
				Output:     int(pc.OutputTokens),
				CacheRead:  int(pc.CacheReadInputTokens),
				CacheWrite: int(pc.CacheCreationInputTokens),
			},
			Source: SourceEventsResult,
		}
	}
}

// ScrollbackPeakCollector wraps panestream.ExtractResponseTokens as an
// output-only floor: Output is the extracted peak, and the input/cache fields it
// cannot observe stay zero (it must not fabricate them). A pane with no token
// marker (peak 0) yields SourceNone.
func ScrollbackPeakCollector(pane string) Collector {
	return func() Result {
		peak := panestream.ExtractResponseTokens(pane)
		if peak == 0 {
			return Result{Source: SourceNone}
		}
		return Result{
			Usage:  cyclestate.TokenUsage{Output: peak},
			Source: SourceScrollbackPeak,
		}
	}
}
