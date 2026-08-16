package bridge

// contextfill_contributors_test.go — WIRING half of the cycle-1482 task
// `context-fill-warning-attribution` RED contract. This is a REACHABILITY
// test, not a unit test: it drives the real production caller
// (Engine.recordTokenUsage) and asserts the contributor breakdown attached to
// the CONTEXT-FILL WARN comes from the SAME basis result.FillPct was derived
// from — never a second, disagreeing total. A test that called
// tokenusage.FillWarnWithContributors directly would pass on dead code.
//
// RED: tokenusage.Result carries no PeakUsage field yet, so this file fails to
// COMPILE until Builder adds it and wires recordTokenUsage to prefer
// result.PeakUsage over result.Usage whenever result.PeakPromptTokens != 0 —
// the same distinction fillpct.go's windowOccupancy already documents for the
// percentage itself (compile-fail = RED evidence).
//
// Reuses runContextFillCase's sibling helpers (contextFillLine) from
// contextfill_warn_test.go, same package.

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

// TestContextFillWarn_ContributorsMatchPeakPromptReading is the cycle-1458 M1
// continuation predicate: the contributor breakdown attached to a fill WARN
// must be measured on the SAME basis as the percentage it annotates — the
// fullest single observed turn — never the whole-launch summed total
// (adversarial-review F1: a 70% reading annotated with contributor figures
// that total far more than the window).
//
// Fixture: an early, large-cache_read turn dominates the whole-launch SUM, but
// a LATER, smaller turn is the actual PEAK (the one the resolver already
// selected for FillPct via windowOccupancy). Only a fix that carries the peak
// turn's own components through to the contributor breakdown can pass.
func TestContextFillWarn_ContributorsMatchPeakPromptReading(t *testing.T) {
	start := time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Second)
	ws := t.TempDir()

	var errBuf bytes.Buffer
	e := NewEngine(Deps{
		Now:                func() time.Time { return end },
		Stderr:             &errBuf,
		ContextFillWarnPct: 60,
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) {
			return tokenusage.Result{
				// Whole-launch summed total: dominated by an EARLIER turn's stale
				// cache_read that no longer reflects what made THIS turn hot.
				Usage: cyclestate.TokenUsage{Input: 1_200, CacheRead: 7_800},
				// The fullest SINGLE turn's own components — smaller input, but its
				// own cache_read alone crosses the window. This is the turn FillPct
				// below is actually derived from.
				PeakUsage:        cyclestate.TokenUsage{Input: 200, CacheRead: 6_800},
				PeakPromptTokens: 7_000,
				Source:           tokenusage.SourceTranscript,
				FillPct:          70.0, // 7000 / 10000 — derived from the peak turn, not the sum
			}, nil
		},
	})
	req := core.BridgeRequest{CLI: "claude-tmux", Agent: "build", Workspace: ws, Worktree: t.TempDir()}
	var resp core.BridgeResponse
	e.recordTokenUsage(req, "sonnet", 0, start, &resp)

	line, ok := contextFillLine(errBuf.String())
	if !ok {
		t.Fatalf("RED: no CONTEXT-FILL WARN at 70%% fill\nstderr:\n%s", errBuf.String())
	}
	if !strings.Contains(line, "cache_read=6800") || !strings.Contains(line, "input=200") {
		t.Errorf("RED: contributors are not the peak turn's own components (want cache_read=6800, input=200): %q", line)
	}
	if strings.Contains(line, "cache_read=7800") || strings.Contains(line, "input=1200") {
		t.Errorf("RED: contributors are the whole-launch SUM, not the peak single-turn reading the percentage is derived from: %q", line)
	}
}

// TestContextFillWarn_ContributorsFallBackToUsageWithoutPeakData is the
// adversarial edge half of the same fix: a tier that never reports a per-turn
// breakdown (events/scrollback — PeakPromptTokens == 0) has only the
// whole-launch total to show, and that total IS already a single reading in
// that case (fillpct.go's windowOccupancy documents the same distinction).
// This pins that the M1 fix must not regress the pre-existing, already-correct
// contributor path for those tiers.
func TestContextFillWarn_ContributorsFallBackToUsageWithoutPeakData(t *testing.T) {
	start := time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Second)
	ws := t.TempDir()

	var errBuf bytes.Buffer
	e := NewEngine(Deps{
		Now:                func() time.Time { return end },
		Stderr:             &errBuf,
		ContextFillWarnPct: 60,
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) {
			return tokenusage.Result{
				Usage:            cyclestate.TokenUsage{Input: 100_000, CacheRead: 20_000},
				PeakPromptTokens: 0, // no per-turn breakdown observed (events/scrollback tier)
				Source:           tokenusage.SourceEventsResult,
				FillPct:          70.0,
			}, nil
		},
	})
	req := core.BridgeRequest{CLI: "codex", Agent: "build", Workspace: ws, Worktree: t.TempDir()}
	var resp core.BridgeResponse
	e.recordTokenUsage(req, "sonnet", 0, start, &resp)

	line, ok := contextFillLine(errBuf.String())
	if !ok {
		t.Fatalf("RED: no CONTEXT-FILL WARN at 70%% fill\nstderr:\n%s", errBuf.String())
	}
	if !strings.Contains(line, "cache_read=20000") || !strings.Contains(line, "input=100000") {
		t.Errorf("RED: without a peak-turn reading, contributors must fall back to the whole-launch total: %q", line)
	}
}
