package tokenusage

import (
	"fmt"
	"strings"
)

type Roots struct {
	Claude string
	Agy    string
}

func DefaultResolver(configRoot string) func(Window) (Result, error) {
	return Resolver(Roots{Claude: configRoot, Agy: AgyDataRoot(nil)}, SQLiteCLI)
}

func Resolver(roots Roots, query SQLiteQuery) func(Window) (Result, error) {
	return func(w Window) (Result, error) {
		r := Chain(driverChain(roots, query, w)...)
		if r.Source == SourceNone {
			r.Warn = fmt.Sprintf("token usage uncovered for driver %q — no transcript/events/scrollback data (fail-open, recorded as unmeasured not zero-cost)", driverLabel(w.Driver))
			r.FillPct = FillPctUnmeasured
			return r, nil
		}
		r.FillPct = FillPct(windowOccupancy(r), EffectiveWindow(w.Driver))
		return r, nil
	}
}

func driverChain(roots Roots, query SQLiteQuery, w Window) []Collector {
	if isClaudeDriver(w.Driver) {
		return []Collector{
			TranscriptCollector(roots.Claude, w),
			EventsResultCollector(w.EventsLogPath),
			ScrollbackPeakCollector(w.Scrollback),
		}
	}
	chain := []Collector{
		EventsResultCollector(w.EventsLogPath),
		ScrollbackPeakCollector(w.Scrollback),
		TokenLinePeakCollector(w.Scrollback, w.TokenLineRegex),
	}
	if strings.HasPrefix(w.Driver, "agy") {
		chain = append([]Collector{AgyConversationCollector(roots.Agy, w, query)}, chain...)
	}
	return chain
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
