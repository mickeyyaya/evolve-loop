package tokenusage

// defaultresolver.go — the single shared resolver both production
// composition roots (internal/adapters/bridge.Adapter and internal/subagent's
// defaultExecAdapter) wire into their gobridge.Deps.TokenResolver field. See
// defaultresolver_test.go for the RED contract this satisfies.
//
// Cycle-779 (token-telemetry-input-cache-fidelity): the chain is dispatched
// per driver. Claude drivers get the full fidelity chain (their transcript
// JSONL is the format ScanConfigRoot parses); every other driver — agy,
// codex, and anything unknown — fails OPEN onto the driver-agnostic tiers
// (events-log result envelope, pane scrollback). A resolve where no tier has
// data carries an explicit per-driver Warn so an uncovered driver surfaces as
// "unmeasured", never as a silent zero recorded as if covered.

import (
	"fmt"
	"strings"
)

// DefaultResolver returns a resolver that recovers token usage for a Window
// through the driver-appropriate fidelity chain: transcript (claude only) >
// eventsResult (w.EventsLogPath) > scrollbackPeak (w.Scrollback). It never
// errors — telemetry is best-effort — and falls open to SourceNone with a
// per-driver coverage Warn when no tier has data.
func DefaultResolver(configRoot string) func(Window) (Result, error) {
	return func(w Window) (Result, error) {
		r := Chain(driverChain(configRoot, w)...)
		if r.Source == SourceNone {
			r.Warn = fmt.Sprintf("token usage uncovered for driver %q — no transcript/events/scrollback data (fail-open, recorded as unmeasured not zero-cost)", driverLabel(w.Driver))
		}
		return r, nil
	}
}

// driverChain returns the fidelity-ordered collectors for a Window's driver.
// Only claude drivers have a Claude Code transcript to scan; all other
// drivers (known or unknown) get the driver-agnostic lower tiers, so an
// unrecognized driver degrades gracefully instead of erroring.
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

// isClaudeDriver reports whether a driver identity names a claude CLI launch
// ("claude", "claude-tmux", …). Empty means claude for backward compatibility
// with pre-driver Windows.
func isClaudeDriver(driver string) bool {
	return driver == "" || driver == "claude" || strings.HasPrefix(driver, "claude-")
}

// driverLabel names a driver in diagnostics; an empty identity is reported as
// the claude default rather than an empty string.
func driverLabel(driver string) string {
	if driver == "" {
		return "claude"
	}
	return driver
}
