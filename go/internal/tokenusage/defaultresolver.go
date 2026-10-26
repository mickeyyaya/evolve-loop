package tokenusage

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/attemptpostmortem"
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
		r := Chain(collectorChain(roots, query, w)...)
		if r.Source == SourceNone {
			r.Warn, r.FillPct = uncoveredWarning(w), FillPctUnmeasured
			if !HasCollector(w.Collector) {
				r.Source = SourceNoCollector
			}
			return r, nil
		}
		r.FillPct = FillPct(windowOccupancy(r), w.ContextWindow)
		return r, nil
	}
}

const (
	FormatClaudeJSONL     = attemptpostmortem.FormatClaudeJSONL
	FormatAgyConversation = "agy-conversation-v1"
)

type formatChain func(roots Roots, query SQLiteQuery, w Window) []Collector

var collectorFormats = map[string]formatChain{
	FormatClaudeJSONL: func(roots Roots, _ SQLiteQuery, w Window) []Collector {
		return []Collector{TranscriptCollector(roots.Claude, w), EventsResultCollector(w.EventsLogPath), ScrollbackPeakCollector(w.Scrollback)}
	},
	FormatAgyConversation: func(roots Roots, query SQLiteQuery, w Window) []Collector {
		return append([]Collector{AgyConversationCollector(roots.Agy, w, query)}, genericChain(w)...)
	},
}

func collectorChain(roots Roots, query SQLiteQuery, w Window) []Collector {
	if chain, ok := collectorFormats[w.Collector]; ok {
		return chain(roots, query, w)
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

const unmeasuredNote = "(fail-open, recorded as unmeasured not zero-cost)"

func HasCollector(format string) bool {
	_, ok := collectorFormats[format]
	return ok
}

func uncoveredWarning(w Window) string {
	if !HasCollector(w.Collector) {
		return fmt.Sprintf("token usage unavailable: driver %q has no usage collector (state %s), and its events and scrollback held no usage %s", w.Driver, SourceNoCollector, unmeasuredNote)
	}
	return fmt.Sprintf("token usage uncovered for driver %q — no transcript/events/scrollback data %s", w.Driver, unmeasuredNote)
}
