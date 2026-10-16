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

func TestContextFillWarn_ContributorsMatchPeakPromptReading(t *testing.T) {
	t.Parallel()
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
				Usage:            cyclestate.TokenUsage{Input: 1_200, CacheRead: 7_800},
				PeakUsage:        cyclestate.TokenUsage{Input: 200, CacheRead: 6_800},
				PeakPromptTokens: 7_000,
				Source:           tokenusage.SourceTranscript,
				FillPct:          70.0,
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

func TestContextFillWarn_ContributorsFallBackToUsageWithoutPeakData(t *testing.T) {
	t.Parallel()
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
				PeakPromptTokens: 0,
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
