// Package phasetiming is the single source for the per-phase latency record
// persisted to <workspace>/phase-timing.json — the durable evidence the loop
// emits to justify where each cycle spends its wall-clock. The orchestrator
// writes Entry values (core aliases its phaseTimingEntry to Entry, so the
// schema is defined once); the dossier producer and the `evolve cycle timing`
// CLI read them back and Rollup() them into a cycle-level summary. Leaf package:
// stdlib only, importable by core, dossier, and cmd without cycles.
package phasetiming

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FileName is the per-cycle timing log written under the cycle workspace.
const FileName = "phase-timing.json"

// Entry is one phase dispatch's terminal latency + outcome record. The JSON
// tags are the on-disk contract (ADR-0044 C1); core's recordPhaseOutcome is the
// sole writer. StartedAt/EndedAt are the wall-clock anchors; Archetype is the
// composition class (plan/build/evaluate/control) the Rollup buckets by.
type Entry struct {
	Phase      string `json:"phase"`
	DurationMS int64  `json:"duration_ms"`
	// BootMS is the cold REPL-boot slice of DurationMS — dispatch overhead before
	// the model worked. The tmux-REPL driver derives it from a Sleep-interval
	// counter (deterministic under the test no-op Sleep, ≈ wall time in
	// production), so values are quantized to whole poll intervals (a ~2s
	// readiness baseline + 1–2s steps) — real, not fabricated, but coarse. 0
	// (omitempty ⇒ absent) on the warm/resumed-session and headless paths, which
	// never cold-boot. Empirically ~0.5% of cycle wall-clock: not a latency lever.
	BootMS       int64   `json:"boot_ms,omitempty"`
	Verdict      string  `json:"verdict"`
	CostUSD      float64 `json:"cost_usd"`
	StartedAt    string  `json:"started_at,omitempty"`
	EndedAt      string  `json:"ended_at,omitempty"`
	Archetype    string  `json:"archetype,omitempty"`
	AttemptCount int     `json:"attempt_count"`
	AbortReason  string  `json:"abort_reason,omitempty"`
}

// Path is the timing-log path for a cycle workspace.
func Path(workspace string) string { return filepath.Join(workspace, FileName) }

// Read parses <workspace>/phase-timing.json. A missing file returns the
// os.ErrNotExist error unwrapped so callers can branch on os.IsNotExist.
func Read(workspace string) ([]Entry, error) {
	data, err := os.ReadFile(Path(workspace))
	if err != nil {
		return nil, err
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse %s: %w", FileName, err)
	}
	return entries, nil
}

// Summary is the cycle-level latency roll-up: where the wall-clock went. It is
// the evidence that answers "which block is longest, and how much is spent
// checking the reports / recovering from errors".
type Summary struct {
	TotalMS      int64            `json:"total_ms"`
	PhaseCount   int              `json:"phase_count"`
	RetriedCount int              `json:"retried_count"` // dispatches that took >1 attempt
	LongestPhase string           `json:"longest_phase"`
	LongestMS    int64            `json:"longest_ms"`
	ByArchetype  map[string]int64 `json:"by_archetype_ms"` // archetype -> summed duration_ms
}

// Rollup aggregates entries into a Summary. Archetype-less entries (legacy logs
// written before the field existed) bucket under "unknown" so totals still sum.
func Rollup(entries []Entry) Summary {
	s := Summary{ByArchetype: map[string]int64{}}
	for _, e := range entries {
		s.TotalMS += e.DurationMS
		s.PhaseCount++
		if e.AttemptCount > 1 {
			s.RetriedCount++
		}
		if e.DurationMS > s.LongestMS {
			s.LongestMS = e.DurationMS
			s.LongestPhase = e.Phase
		}
		arch := e.Archetype
		if arch == "" {
			arch = "unknown"
		}
		s.ByArchetype[arch] += e.DurationMS
	}
	return s
}

// ArchetypePercent is the share (0–100) of total wall-clock in an archetype.
func (s Summary) ArchetypePercent(archetype string) float64 {
	if s.TotalMS == 0 {
		return 0
	}
	return float64(s.ByArchetype[archetype]) / float64(s.TotalMS) * 100
}

// HumanMS renders a millisecond duration compactly (e.g. 13m4s, 6m40s, 2s) —
// the shared formatter for both the CLI table and the dossier markdown.
func HumanMS(ms int64) string {
	d := (time.Duration(ms) * time.Millisecond).Round(time.Second)
	if d == 0 {
		return "0s"
	}
	return d.String()
}
