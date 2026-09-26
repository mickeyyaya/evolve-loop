package bridge

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

// contextFillMarker is the stable grep key the WARN line must carry: keying on
// the agent name alone would false-green on the pre-existing per-driver
// coverage WARN, which also names the agent.
var contextFillMarker = string(CodeContextFillHigh)

func runContextFillCase(t *testing.T, fill float64, warnPct int, agent string) (stderr, record string) {
	t.Helper()
	start := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Second)
	ws := t.TempDir()

	var errBuf bytes.Buffer
	e := NewEngine(Deps{
		Now:                func() time.Time { return end },
		Stderr:             &errBuf,
		Signals:            sinkDeps(&errBuf),
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

func contextFillLine(stderr string) (string, bool) {
	for _, ln := range strings.Split(stderr, "\n") {
		if strings.Contains(ln, contextFillMarker) {
			return ln, true
		}
	}
	return "", false
}

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

// Three ways a WARN must not fire, each of which a naive `>=` comparison or a
// missing sentinel guard would break: exactly at threshold, below it, and
// unmeasured.
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

// An unconfigured ContextFillWarnPct (the zero value every existing
// composition path leaves) must resolve to 60, matching policy's built-in —
// not to 0, which would warn on every launch.
func TestContextFillWarn_ZeroDepsResolvesToDefaultThreshold(t *testing.T) {
	if stderr, _ := runContextFillCase(t, 59, 0, "scout"); func() bool { _, ok := contextFillLine(stderr); return ok }() {
		t.Errorf("59%% fill warned under the zero-value threshold — zero must resolve to 60, not to 0")
	}
	stderr, _ := runContextFillCase(t, 61, 0, "scout")
	if _, ok := contextFillLine(stderr); !ok {
		t.Errorf("61%% fill did not warn under the zero-value threshold — zero must resolve to 60.\nstderr:\n%s", stderr)
	}
}

// fill_pct must persist in the llm-calls record, not merely print to stderr:
// the deferred fill%-vs-verdict correlation report has no corpus without it.
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
