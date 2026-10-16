package tokenusage

import (
	"fmt"
	"strings"
)

func DefaultResolver(configRoot string) func(Window) (Result, error) {
	return func(w Window) (Result, error) {
		r := Chain(driverChain(configRoot, w)...)
		if r.Source == SourceNone {
			r.Warn = fmt.Sprintf("token usage uncovered for driver %q — no transcript/events/scrollback data (fail-open, recorded as unmeasured not zero-cost)", driverLabel(w.Driver))
			r.FillPct = FillPctUnmeasured
			return r, nil
		}
		r.FillPct = FillPct(windowOccupancy(r), EffectiveWindow(w.Driver))
		return r, nil
	}
}

func driverChain(configRoot string, w Window) []Collector {
	if isClaudeDriver(w.Driver) {
		return []Collector{
			TranscriptCollector(configRoot, w),
			EventsResultCollector(w.EventsLogPath),
			ScrollbackPeakCollector(w.Scrollback),
		}
	}
	return []Collector{
		EventsResultCollector(w.EventsLogPath),
		ScrollbackPeakCollector(w.Scrollback),
		TokenLinePeakCollector(w.Scrollback, w.TokenLineRegex),
	}
}

func isClaudeDriver(driver string) bool {
	return driver == "" || driver == "claude" || strings.HasPrefix(driver, "claude-")
}

func driverLabel(driver string) string {
	if driver == "" {
		return "claude"
	}
	return driver
}
