// Package phasetiming is the single source for the per-phase latency record
// persisted to <workspace>/phase-timing.json — the durable evidence the
// loop emits to justify where each cycle spends its wall-clock.
package phasetiming

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/contextfill"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

const FileName = "phase-timing.json"

type Entry struct {
	Phase            string                  `json:"phase"`
	DurationMS       int64                   `json:"duration_ms"`
	BootMS           int64                   `json:"boot_ms,omitempty"`
	Verdict          string                  `json:"verdict"`
	CostUSD          float64                 `json:"cost_usd"`
	StartedAt        string                  `json:"started_at,omitempty"`
	EndedAt          string                  `json:"ended_at,omitempty"`
	Archetype        string                  `json:"archetype,omitempty"`
	AttemptCount     int                     `json:"attempt_count"`
	AbortReason      string                  `json:"abort_reason,omitempty"`
	ModelSource      string                  `json:"model_source,omitempty"`
	ResolvedModel    string                  `json:"resolved_model,omitempty"`
	Tokens           cyclestate.TokenUsage   `json:"tokens,omitempty"`
	ContextFillRatio float64                 `json:"context_fill_ratio,omitempty"`
	ContextWindowHot bool                    `json:"context_window_hot,omitempty"`
	Diagnostics      []cyclestate.Diagnostic `json:"diagnostics,omitempty"`
}

func Path(workspace string) string { return filepath.Join(workspace, FileName) }

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

type Summary struct {
	TotalMS           int64                            `json:"total_ms"`
	PhaseCount        int                              `json:"phase_count"`
	RetriedCount      int                              `json:"retried_count"`
	LongestPhase      string                           `json:"longest_phase"`
	LongestMS         int64                            `json:"longest_ms"`
	ByArchetype       map[string]int64                 `json:"by_archetype_ms"`
	TotalTokens       cyclestate.TokenUsage            `json:"total_tokens"`
	TokensByArchetype map[string]cyclestate.TokenUsage `json:"tokens_by_archetype"`
	WastedTokens      cyclestate.TokenUsage            `json:"wasted_tokens"`
	CacheHitRatio     float64                          `json:"cache_hit_ratio"`
	HotPhaseCount     int                              `json:"hot_phase_count"`
	HotPhases         []string                         `json:"hot_phases,omitempty"`
}

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
		if contextfill.IsHot(e.ContextFillRatio) {
			s.HotPhaseCount++
			s.HotPhases = append(s.HotPhases, e.Phase)
		}
	}
	if denom := s.TotalTokens.Input + s.TotalTokens.CacheRead; denom > 0 {
		s.CacheHitRatio = float64(s.TotalTokens.CacheRead) / float64(denom)
	}
	return s
}

func addTokens(a, b cyclestate.TokenUsage) cyclestate.TokenUsage {
	a.Input += b.Input
	a.Output += b.Output
	a.CacheRead += b.CacheRead
	a.CacheWrite += b.CacheWrite
	return a
}

func (s Summary) ArchetypePercent(archetype string) float64 {
	if s.TotalMS == 0 {
		return 0
	}
	return float64(s.ByArchetype[archetype]) / float64(s.TotalMS) * 100
}

func HumanMS(ms int64) string {
	d := (time.Duration(ms) * time.Millisecond).Round(time.Second)
	if d == 0 {
		return "0s"
	}
	return d.String()
}

type ParallelProjection struct {
	GroupPhases         []string `json:"group_phases"`
	Concurrency         int      `json:"concurrency"`
	SequentialMS        int64    `json:"sequential_ms"`
	ProjectedParallelMS int64    `json:"projected_parallel_ms"`
	SavingMS            int64    `json:"saving_ms"`
}

func ProjectParallelSaving(entries []Entry, concurrency int) ParallelProjection {
	p := ParallelProjection{Concurrency: concurrency}
	var sum, longest int64
	for _, e := range entries {
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

func ceilDiv(a, b int64) int64 { return (a + b - 1) / b }
