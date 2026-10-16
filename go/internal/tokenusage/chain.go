package tokenusage

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclecost"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

const (
	SourceEventsResult   Source = "events_result"
	SourceScrollbackPeak Source = "scrollback_peak"
)

type Collector func() Result

func Chain(collectors ...Collector) Result {
	for _, c := range collectors {
		if r := c(); r.Source != SourceNone {
			return r
		}
	}
	return Result{Source: SourceNone}
}

func TranscriptCollector(root string, w Window) Collector {
	return func() Result {
		if root == "" {
			return Result{Source: SourceNone}
		}
		r, err := ScanConfigRoot(root, w)
		if err != nil || r.Source == SourceNone {
			return Result{Source: SourceNone}
		}
		return r
	}
}

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

func TokenLinePeakCollector(pane, pattern string) Collector {
	return func() Result {
		peak := panestream.TokenLinePeak(pane, pattern)
		if peak == 0 {
			return Result{Source: SourceNone}
		}
		return Result{Usage: cyclestate.TokenUsage{Output: peak}, Source: SourceScrollbackPeak}
	}
}
