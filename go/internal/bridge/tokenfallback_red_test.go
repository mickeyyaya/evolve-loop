package bridge

// tokenfallback_red_test.go — RED contract for cycle-754 task
// token-resolver-production-wiring (composition-root half; the tokenusage half
// is internal/tokenusage/fallbackchain_test.go).
//
// recordTokenUsage (engine.go) currently builds a tokenusage.Window carrying
// only Worktree/ArtifactPath/Start/End, so the lower fallback tiers can never
// fire and every tmux-driven launch records "source":"none" with zero tokens
// (confirmed live across 124 .evolve/runs/*/llm-calls.ndjson files). This
// contract requires the engine to thread the launch's events-log context —
// the workspace's <agent>-events.ndjson — into the Window it hands the
// resolver, so the REAL production resolver (tokenusage.DefaultResolver)
// recovers usage for a launch with no transcript. DO NOT modify these tests;
// make them pass by wiring context into the Window at the call site.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

// tokenLaunchFixture returns a request whose workspace holds a
// tdd-events.ndjson result envelope (in=900 out=210 cache_r=30 cache_c=7) —
// the exact artifact shape a real tmux launch leaves behind — and an engine
// wired with the REAL production resolver over an empty (transcript-less)
// config root.
func tokenLaunchFixture(t *testing.T, withEventsLog bool) (*Engine, core.BridgeRequest, time.Time) {
	t.Helper()
	ws := t.TempDir()
	if withEventsLog {
		envelope := `{"kind":"result","data":{"cost_usd":0.4,"tokens":{"in":900,"out":210,"cache_r":30,"cache_c":7}}}` + "\n"
		if err := os.WriteFile(filepath.Join(ws, "tdd-events.ndjson"), []byte(envelope), 0o644); err != nil {
			t.Fatalf("write events fixture: %v", err)
		}
	}
	start := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	e := NewEngine(Deps{
		Now:           func() time.Time { return start.Add(90 * time.Second) },
		TokenResolver: tokenusage.DefaultResolver(t.TempDir()), // empty root: no transcript tier data
	})
	req := core.BridgeRequest{
		CLI:          "claude-tmux",
		Agent:        "tdd",
		Workspace:    ws,
		Worktree:     "/repo/worktrees/cycle-754",
		ArtifactPath: filepath.Join(ws, "test-report.md"),
	}
	return e, req, start
}

// readLLMCalls returns the raw contents of the workspace's llm-calls.ndjson.
func readLLMCalls(t *testing.T, workspace string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(workspace, "llm-calls.ndjson"))
	if err != nil {
		t.Fatalf("llm-calls.ndjson not written: %v", err)
	}
	return string(b)
}

// TestRecordTokenUsage_EventsLogFallback_EndToEnd — AC2/AC5 (hermetic form).
// A tmux-style launch with NO transcript but a present workspace
// <agent>-events.ndjson must record REAL token usage via the eventsResult
// tier: resp.Tokens carries the envelope counts and the llm-calls.ndjson
// record says "source":"events_result" — not the permanently-zero
// "source":"none" the production fleet shows today.
func TestRecordTokenUsage_EventsLogFallback_EndToEnd(t *testing.T) {
	e, req, start := tokenLaunchFixture(t, true)

	var resp core.BridgeResponse
	e.recordTokenUsage(req, "sonnet", 0, start, &resp)

	if resp.Tokens.Output != 210 || resp.Tokens.Input != 900 {
		t.Errorf("resp.Tokens = %+v, want Input=900 Output=210 recovered from the workspace events log", resp.Tokens)
	}
	rec := readLLMCalls(t, req.Workspace)
	if !strings.Contains(rec, `"source":"events_result"`) {
		t.Errorf("llm-calls.ndjson must attribute the eventsResult tier, got: %s", rec)
	}
	if strings.Contains(rec, `"source":"none"`) {
		t.Errorf("launch with a recoverable events log must not record source=none, got: %s", rec)
	}
}

// TestRecordTokenUsage_NoSources_RecordsSourceNoneZeroTokens — AC3 negative
// (anti-fabrication guard; expected pre-existing GREEN). A launch with no
// transcript, no events log, and no scrollback data must keep recording
// "source":"none" with zero tokens. An implementation that stamps a
// non-none source or invents counts to satisfy the e2e test must fail here.
func TestRecordTokenUsage_NoSources_RecordsSourceNoneZeroTokens(t *testing.T) {
	e, req, start := tokenLaunchFixture(t, false)

	var resp core.BridgeResponse
	e.recordTokenUsage(req, "sonnet", 0, start, &resp)

	if resp.Tokens != (core.TokenUsage{}) {
		t.Errorf("no telemetry source available: resp.Tokens must stay zero (no fabrication), got %+v", resp.Tokens)
	}
	rec := readLLMCalls(t, req.Workspace)
	if !strings.Contains(rec, `"source":"none"`) {
		t.Errorf("no-source launch must record source=none, got: %s", rec)
	}
}
