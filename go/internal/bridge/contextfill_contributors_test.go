package bridge

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

// Fixture: an early, large-cache_read turn dominates the whole-launch sum, but
// a later, smaller turn is the actual peak (the one the resolver already
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
		Signals:            sinkDeps(&errBuf),
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

// A tier with no per-turn breakdown (PeakPromptTokens == 0) has only the
// whole-launch total, which fillpct.go's windowOccupancy already treats as a
// single reading in that case.
func TestContextFillWarn_ContributorsFallBackToUsageWithoutPeakData(t *testing.T) {
	start := time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Second)
	ws := t.TempDir()

	var errBuf bytes.Buffer
	e := NewEngine(Deps{
		Now:                func() time.Time { return end },
		Stderr:             &errBuf,
		Signals:            sinkDeps(&errBuf),
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
