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
			r.Warn = uncoveredWarning(w.Driver)
			r.FillPct = FillPctUnmeasured
			return r, nil
		}
		r.FillPct = FillPct(windowOccupancy(r), EffectiveWindow(w.Driver))
		return r, nil
	}
}

type collectorFamily struct {
	matches func(driver string) bool
	chain   func(roots Roots, query SQLiteQuery, w Window) []Collector
}

var collectorFamilies = []collectorFamily{
	{matches: isClaudeDriver, chain: func(roots Roots, _ SQLiteQuery, w Window) []Collector {
		return []Collector{TranscriptCollector(roots.Claude, w), EventsResultCollector(w.EventsLogPath), ScrollbackPeakCollector(w.Scrollback)}
	}},
	{matches: isAgyDriver, chain: func(roots Roots, query SQLiteQuery, w Window) []Collector {
		return append([]Collector{AgyConversationCollector(roots.Agy, w, query)}, genericChain(w)...)
	}},
}

func driverChain(roots Roots, query SQLiteQuery, w Window) []Collector {
	if f, ok := familyOf(w.Driver); ok {
		return f.chain(roots, query, w)
	}
	return genericChain(w)
}

func genericChain(w Window) []Collector {
	return []Collector{
		EventsResultCollector(w.EventsLogPath),
		ScrollbackPeakCollector(w.Scrollback),
		TokenLinePeakCollector(w.Scrollback, w.TokenLineRegex),
	}
}

func familyOf(driver string) (collectorFamily, bool) {
	for _, f := range collectorFamilies {
		if f.matches(driver) {
			return f, true
		}
	}
	return collectorFamily{}, false
}

const unmeasuredNote = "(fail-open, recorded as unmeasured not zero-cost)"

func isClaudeDriver(driver string) bool {
	d := strings.ToLower(driver)
	return d == "" || d == "claude" || strings.HasPrefix(d, "claude-")
}

func isAgyDriver(driver string) bool {
	d := strings.ToLower(driver)
	return d == "agy" || strings.HasPrefix(d, "agy-")
}

func HasCollector(driver string) bool {
	_, ok := familyOf(driver)
	return ok
}

func uncoveredWarning(driver string) string {
	if !HasCollector(driver) {
		return fmt.Sprintf("token usage unavailable: driver %q has no usage collector, and its events and scrollback held no usage %s", driverLabel(driver), unmeasuredNote)
	}
	return fmt.Sprintf("token usage uncovered for driver %q — no transcript/events/scrollback data %s", driverLabel(driver), unmeasuredNote)
}

func driverLabel(driver string) string {
	if driver == "" {
		return "claude"
	}
	return driver
}
