package tokenusage

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclecost"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

const (
	// SourceEventsResult means usage came from a phase's *-events.ndjson result envelope.
	SourceEventsResult Source = "events_result"
	// SourceScrollbackPeak means usage is the pane scrollback's peak response-token marker: output only.
	SourceScrollbackPeak Source = "scrollback_peak"
)

// Collector is a lazily evaluated usage source that returns SourceNone when it has no data.
type Collector func() Result

// Chain returns the first Result, in collector order, whose Source is not SourceNone.
func Chain(collectors ...Collector) Result {
	for _, c := range collectors {
		if r := c(); r.Source != SourceNone {
			return r
		}
	}
	return Result{Source: SourceNone}
}

// TranscriptCollector is the highest-fidelity tier; a scan error or no match reports SourceNone.
func TranscriptCollector(root string, w Window) Collector {
	return func() Result {
		r, err := ScanConfigRoot(root, w)
		if err != nil || r.Source == SourceNone {
			return Result{Source: SourceNone}
		}
		return r
	}
}

// EventsResultCollector parses with cyclecost.ParseEventsLog so its counts match cyclecost's by construction.
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

// ScrollbackPeakCollector is the output-only floor tier; it leaves the input and cache counts it cannot see at zero.
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
