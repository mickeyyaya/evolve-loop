package bridge

// contextfill_warn_test.go — WIRING proof for cycle-1444 task
// `context-fill-warn-threshold`. This is a REACHABILITY test, not a unit test:
// it drives the real production caller (Engine.recordTokenUsage, engine.go:640 —
// the one site every Launch funnels its token telemetry through) and asserts the
// fill WARN and the persisted fill_pct both come out of THAT path. A test that
// called tokenusage.FillWarn directly would pass on dead code.
//
// RED: Deps.ContextFillWarnPct, tokenusage.Result.FillPct, the llm-calls
// fill_pct field and the WARN emission do not exist yet — this file fails to
// COMPILE until Builder adds them (compile-fail = RED evidence).

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

// contextFillMarker is the stable grep key the WARN line must carry. The
// pre-existing per-driver coverage WARN already names the agent, so keying on
// the agent name alone would false-green on that older line; only a distinct
// marker proves the fill WARN specifically fired.
const contextFillMarker = "CONTEXT-FILL"

// runContextFillCase drives recordTokenUsage with a resolver stubbed to report a
// known fill reading, and returns the captured stderr plus the appended
// llm-calls.ndjson contents. readLLMCalls comes from tokenfallback_red_test.go
// (same package).
func runContextFillCase(t *testing.T, fill float64, warnPct int, agent string) (stderr, record string) {
	t.Helper()
	start := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Second)
	ws := t.TempDir()

	var errBuf bytes.Buffer
	e := NewEngine(Deps{
		Now:                func() time.Time { return end },
		Stderr:             &errBuf,
		ContextFillWarnPct: warnPct,
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) {
			return tokenusage.Result{
				Usage:   cyclestate.TokenUsage{Input: 100_000, Output: 500, CacheRead: 20_000},
				Source:  tokenusage.SourceEventsResult,
				FillPct: fill,
			}, nil
		},
	})
	req := core.BridgeRequest{CLI: "claude-tmux", Agent: agent, Workspace: ws, Worktree: t.TempDir()}
	var resp core.BridgeResponse
	e.recordTokenUsage(req, "sonnet", 0, start, &resp)
	return errBuf.String(), readLLMCalls(t, ws)
}

// contextFillLine returns the first stderr line carrying the fill marker.
func contextFillLine(stderr string) (string, bool) {
	for _, ln := range strings.Split(stderr, "\n") {
		if strings.Contains(ln, contextFillMarker) {
			return ln, true
		}
	}
	return "", false
}

// TestContextFillWarn_EmittedAtDispatchNamingPhase is the crux reachability
// assertion: a launch resolving above the threshold must produce a WARN from the
// production telemetry path, and that WARN must name the phase.
func TestContextFillWarn_EmittedAtDispatchNamingPhase(t *testing.T) {
	stderr, _ := runContextFillCase(t, 91.4, 60, "build")
	line, ok := contextFillLine(stderr)
	if !ok {
		t.Fatalf("no %s WARN on a 91.4%% fill — the fill instrument is not reached from recordTokenUsage.\nstderr:\n%s", contextFillMarker, stderr)
	}
	if !strings.Contains(line, "build") {
		t.Errorf("WARN %q does not name the phase — an unattributed fill WARN cannot be acted on", line)
	}
}

// TestContextFillWarn_BoundaryAndSentinelStaySilent is the negative half. Three
// ways a WARN must NOT fire, each of which a naive `>=` or a missing sentinel
// guard would break: exactly at threshold, below it, and unmeasured.
func TestContextFillWarn_BoundaryAndSentinelStaySilent(t *testing.T) {
	cases := []struct {
		name string
		fill float64
	}{
		{"exactly at threshold", 60},
		{"below threshold", 59.9},
		{"unmeasured sentinel", tokenusage.FillPctUnmeasured},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stderr, _ := runContextFillCase(t, c.fill, 60, "build")
			if line, ok := contextFillLine(stderr); ok {
				t.Errorf("fill=%v produced a WARN it must not: %q", c.fill, line)
			}
		})
	}
}

// TestContextFillWarn_ZeroDepsResolvesToDefaultThreshold pins the Deps default:
// an unconfigured ContextFillWarnPct (the zero value every existing composition
// path leaves) must behave as 60, matching policy's built-in — not as 0, which
// would warn on every single launch.
func TestContextFillWarn_ZeroDepsResolvesToDefaultThreshold(t *testing.T) {
	if stderr, _ := runContextFillCase(t, 59, 0, "scout"); func() bool { _, ok := contextFillLine(stderr); return ok }() {
		t.Errorf("59%% fill warned under the zero-value threshold — zero must resolve to 60, not to 0")
	}
	stderr, _ := runContextFillCase(t, 61, 0, "scout")
	if _, ok := contextFillLine(stderr); !ok {
		t.Errorf("61%% fill did not warn under the zero-value threshold — zero must resolve to 60.\nstderr:\n%s", stderr)
	}
}

// TestContextFillWarn_PersistedInLLMCallsRecord proves the reading is DURABLE,
// not merely printed: the deferred fill%-vs-verdict correlation report has no
// corpus to read unless every launch record carries fill_pct.
func TestContextFillWarn_PersistedInLLMCallsRecord(t *testing.T) {
	_, record := runContextFillCase(t, 72.5, 60, "audit")
	var rec struct {
		Phase   string   `json:"phase"`
		FillPct *float64 `json:"fill_pct"`
	}
	line := strings.TrimSpace(record)
	if i := strings.LastIndex(line, "\n"); i >= 0 {
		line = line[i+1:]
	}
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("llm-calls record is not JSON: %v\nrecord: %s", err, record)
	}
	if rec.FillPct == nil {
		t.Fatalf("llm-calls record has no fill_pct field — fill telemetry is printed but never persisted, so the correlation corpus stays empty.\nrecord: %s", record)
	}
	if *rec.FillPct != 72.5 {
		t.Errorf("fill_pct = %v, want 72.5", *rec.FillPct)
	}
}
