// Package phasetiming is the single source for the per-phase latency record
// persisted to <workspace>/phase-timing.json — the durable evidence the loop
// emits to justify where each cycle spends its wall-clock. The orchestrator
// writes Entry values (core aliases its phaseTimingEntry to Entry, so the
// schema is defined once); the dossier producer and the `evolve cycle timing`
// CLI read them back and Rollup() them into a cycle-level summary. Leaf package:
// stdlib plus the zero-dependency cyclestate value types (TokenUsage) —
// importable by core, dossier, and cmd without cycles.
package phasetiming

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
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
	// ModelSource + ResolvedModel (T3, cycle-463) mirror core.PhaseResponse's
	// per-phase model provenance into the timing log, so the dossier (which
	// ingests this file, not the live PhaseResponse) can project WHICH
	// resolution path won — "profile"|"pin"|"advisor" — plus the concrete
	// resolved model/tier. Both empty on a legacy log written before this
	// change; that degrades to "absent", never a fabricated claim.
	ModelSource   string `json:"model_source,omitempty"`
	ResolvedModel string `json:"resolved_model,omitempty"`
	// Tokens (S4, token-telemetry) is the TERMINAL attempt's LLM token usage,
	// projected here beside CostUSD so the durable per-phase record carries
	// counts, not just dollars. Per-attempt detail already lives in
	// llm-calls.ndjson (S3); this is the single terminal number the dossier
	// rolls up. A legacy log written before this field parses to a zero value
	// (never an error) — that degrades to "absent", never a fabricated count.
	Tokens cyclestate.TokenUsage `json:"tokens,omitempty"`
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
	// Token rollups (S6, token-telemetry): the cycle-level twin of the duration
	// rollup. TotalTokens sums every entry's terminal token usage;
	// TokensByArchetype buckets it the same way ByArchetype buckets duration;
	// WastedTokens is the usage spent by FAIL-verdict entries (work that did not
	// ship); CacheHitRatio is cache_read/(input+cache_read) over the whole cycle
	// (0 when there was no input at all). All zero on a legacy log with no token
	// fields — degrades to "absent", never a fabricated count.
	TotalTokens       cyclestate.TokenUsage            `json:"total_tokens"`
	TokensByArchetype map[string]cyclestate.TokenUsage `json:"tokens_by_archetype"`
	WastedTokens      cyclestate.TokenUsage            `json:"wasted_tokens"`
	CacheHitRatio     float64                          `json:"cache_hit_ratio"`
}

// Rollup aggregates entries into a Summary. Archetype-less entries (legacy logs
// written before the field existed) bucket under "unknown" so totals still sum.
func Rollup(entries []Entry) Summary {
	s := Summary{
		ByArchetype:       map[string]int64{},
		TokensByArchetype: map[string]cyclestate.TokenUsage{},
	}
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
		s.TotalTokens = addTokens(s.TotalTokens, e.Tokens)
		s.TokensByArchetype[arch] = addTokens(s.TokensByArchetype[arch], e.Tokens)
		if e.Verdict == "FAIL" {
			s.WastedTokens = addTokens(s.WastedTokens, e.Tokens)
		}
	}
	if denom := s.TotalTokens.Input + s.TotalTokens.CacheRead; denom > 0 {
		s.CacheHitRatio = float64(s.TotalTokens.CacheRead) / float64(denom)
	}
	return s
}

// addTokens sums two TokenUsage values field-wise — the shared accumulator for
// the cycle-level and per-archetype token rollups.
func addTokens(a, b cyclestate.TokenUsage) cyclestate.TokenUsage {
	a.Input += b.Input
	a.Output += b.Output
	a.CacheRead += b.CacheRead
	a.CacheWrite += b.CacheWrite
	return a
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

// ParallelProjection is the SHADOW estimate (ADR shadow-stage discipline) of
// parallelizing the independent post-build checking phases: their sequential
// wall-clock vs the makespan if run concurrently at Concurrency. It is pure
// projection from recorded durations — no execution change — and the evidence
// that justifies the enforce-stage dispatcher (PR2b). SavingMS is a best-case
// figure (makespan lower bound; real scheduling cannot beat it).
type ParallelProjection struct {
	GroupPhases         []string `json:"group_phases"`
	Concurrency         int      `json:"concurrency"`
	SequentialMS        int64    `json:"sequential_ms"`
	ProjectedParallelMS int64    `json:"projected_parallel_ms"`
	SavingMS            int64    `json:"saving_ms"`
}

// ProjectParallelSaving identifies the parallelizable checking group — the
// archetype "evaluate" phases EXCLUDING audit (the verdict-branching anchor that
// must stay serial) — and projects its makespan at the given concurrency.
//
// The estimate is the standard makespan lower bound max(longestPhase,
// ceil(sum/concurrency)): with concurrency ≥ group size it is the single longest
// phase; with concurrency 1 (or <2 parallelizable phases) there is no saving.
// The projection assumes the evaluate phases are mutually independent (the
// common case — each reads the same immutable build output); a real After-edge
// between two of them would make this an upper bound on the achievable saving.
func ProjectParallelSaving(entries []Entry, concurrency int) ParallelProjection {
	p := ParallelProjection{Concurrency: concurrency}
	var sum, longest int64
	for _, e := range entries {
		// "audit" is excluded by name: it is the verdict-branching anchor that
		// must stay serial, and Entry carries no serial-anchor field.
		if e.Archetype != "evaluate" || e.Phase == "audit" {
			continue
		}
		p.GroupPhases = append(p.GroupPhases, e.Phase)
		sum += e.DurationMS
		if e.DurationMS > longest {
			longest = e.DurationMS
		}
	}
	p.SequentialMS = sum
	// No parallelism possible: fewer than 2 phases, or concurrency ≤ 1.
	if len(p.GroupPhases) < 2 || concurrency <= 1 {
		p.ProjectedParallelMS = sum
		return p
	}
	makespan := ceilDiv(sum, int64(concurrency))
	if longest > makespan {
		makespan = longest
	}
	p.ProjectedParallelMS = makespan
	p.SavingMS = sum - makespan
	return p
}

// ceilDiv returns ceil(a/b) for non-negative a and positive b.
func ceilDiv(a, b int64) int64 { return (a + b - 1) / b }
