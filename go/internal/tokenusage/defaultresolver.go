package tokenusage

import (
	"fmt"
	"strings"
)

// DefaultResolver returns the resolver every composition root wires; it never errors and warns per driver when no tier has data.
func DefaultResolver(configRoot string) func(Window) (Result, error) {
	return func(w Window) (Result, error) {
		r := Chain(driverChain(configRoot, w)...)
		if r.Source == SourceNone {
			r.Warn = fmt.Sprintf("token usage uncovered for driver %q — no transcript/events/scrollback data (fail-open, recorded as unmeasured not zero-cost)", driverLabel(w.Driver))
			r.FillPct = FillPctUnmeasured
			return r, nil
		}
		// Fill derives from this resolve's usage, never a second scan; windowOccupancy picks one turn, not the sum.
		r.FillPct = FillPct(windowOccupancy(r), EffectiveWindow(w.Driver))
		return r, nil
	}
}

// driverChain returns the fidelity-ordered collectors; only claude drivers write a transcript to scan.
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
	}
}

// isClaudeDriver treats an empty driver as claude, the default for Windows built before Driver existed.
func isClaudeDriver(driver string) bool {
	return driver == "" || driver == "claude" || strings.HasPrefix(driver, "claude-")
}

func driverLabel(driver string) string {
	if driver == "" {
		return "claude"
	}
	return driver
}
