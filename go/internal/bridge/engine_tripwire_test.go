package bridge

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

type tripwireCase struct {
	cli        string // driver identity (req.CLI)
	agent      string // req.Agent
	code       int    // launch exit code
	durSecs    int    // wall-clock duration of the launch (end - start)
	covered    bool   // true => a workspace events log makes the resolve source=events_result
	cycleInDir bool   // true => workspace lives under .evolve/runs/cycle-1005
}

// runTripwireCase reuses readLLMCalls from tokenfallback_red_test.go (same
// package).
func runTripwireCase(t *testing.T, c tripwireCase) (stderr, record string) {
	t.Helper()
	start := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Duration(c.durSecs) * time.Second)

	ws := t.TempDir()
	if c.cycleInDir {
		ws = filepath.Join(ws, ".evolve", "runs", "cycle-1005")
		if err := os.MkdirAll(ws, 0o755); err != nil {
			t.Fatalf("mkdir workspace: %v", err)
		}
	}
	if c.covered {
		// A result envelope in <agent>-events.ndjson makes the driver-agnostic
		// eventsResult tier fire, so the resolve is source=events_result
		// (covered), not source=none, and the tripwire must stay silent.
		envelope := `{"kind":"result","data":{"cost_usd":0.4,"tokens":{"in":900,"out":210,"cache_r":30,"cache_c":7}}}` + "\n"
		if err := os.WriteFile(filepath.Join(ws, c.agent+"-events.ndjson"), []byte(envelope), 0o644); err != nil {
			t.Fatalf("write events fixture: %v", err)
		}
	}

	// See ADR-0101.
	// errBuf holds what the root's WARN-filtered stderr sink renders for the
	// engine's signals — the same one-line format the operator reads.
	var errBuf bytes.Buffer
	signals := signalcenter.New()
	signals.Subscribe(signalcenter.Filter(signalcenter.StderrSink(&errBuf), signalcenter.SeverityWarn))
	e := NewEngine(Deps{
		Now:           func() time.Time { return end },
		Stderr:        io.Discard,
		Signals:       signals,
		TokenResolver: tokenusage.DefaultResolver(t.TempDir()), // empty root: no transcript tier
	})
	req := core.BridgeRequest{
		CLI:       c.cli,
		Agent:     c.agent,
		Workspace: ws,
		Worktree:  "/repo/worktrees/cycle-1005",
	}
	if c.cycleInDir {
		// The dispatch identity comes from the request: the cycle the
		// workspace path names is the one the dispatcher stamped.
		req.Cycle = 1005
	}
	var resp core.BridgeResponse
	e.recordTokenUsage(req, "sonnet", c.code, start, &resp)
	return errBuf.String(), readLLMCalls(t, ws)
}

// Keying on the TRIPWIRE marker is essential: the pre-existing generic
// coverage WARN already contains the CLI and agent names, so a substring
// check for those alone would false-green on it.
func tripwireStderrLine(stderr string) (string, bool) {
	for _, ln := range strings.Split(stderr, "\n") {
		if strings.Contains(ln, "TRIPWIRE") {
			return ln, true
		}
	}
	return "", false
}

func TestRecordTokenUsage_Tripwire_NonClaudeExit0Success_Warns(t *testing.T) {
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 0, durSecs: 90, cycleInDir: true,
	})

	line, ok := tripwireStderrLine(stderr)
	if !ok {
		t.Fatalf("expected a TRIPWIRE escalation line for an exit-0 >60s non-claude source=none launch; stderr:\n%s", stderr)
	}
	for _, needle := range []string{"cli=agy", "agent=builder", "cycle=1005"} {
		if !strings.Contains(line, needle) {
			t.Errorf("TRIPWIRE line must name %q (CLI+agent+cycle contract), got: %s", needle, line)
		}
	}
}

func TestRecordTokenUsage_Tripwire_QuotaAbortShortLaunch_Silent(t *testing.T) {
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 85, durSecs: 5, cycleInDir: true,
	})

	if line, ok := tripwireStderrLine(stderr); ok {
		t.Errorf("exit-85 short quota-abort must NOT trip the tripwire (false positive), got: %s", line)
	}
}

func TestRecordTokenUsage_Tripwire_Exit0ShortDuration_Silent(t *testing.T) {
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 0, durSecs: 5, cycleInDir: true,
	})

	if line, ok := tripwireStderrLine(stderr); ok {
		t.Errorf("exit-0 but sub-threshold duration must NOT trip the tripwire, got: %s", line)
	}
}

func TestRecordTokenUsage_Tripwire_ClaudeBaseline_Silent(t *testing.T) {
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "claude-tmux", agent: "builder", code: 0, durSecs: 90, cycleInDir: true,
	})

	if line, ok := tripwireStderrLine(stderr); ok {
		t.Errorf("claude is the measured baseline, not the tripwire target; must stay silent, got: %s", line)
	}
}

func TestRecordTokenUsage_Tripwire_CoveredNonClaude_Silent(t *testing.T) {
	stderr, record := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 0, durSecs: 90, covered: true, cycleInDir: true,
	})

	// Guard the fixture: this case must actually be covered, else the negative
	// would pass for the wrong reason (source=none rather than "measured").
	if !strings.Contains(record, `"source":"events_result"`) {
		t.Fatalf("fixture invalid: expected a covered (events_result) resolve, got record: %s", record)
	}
	if line, ok := tripwireStderrLine(stderr); ok {
		t.Errorf("a measured (source=events_result) non-claude success must NOT trip the tripwire, got: %s", line)
	}
}

func TestRecordTokenUsage_Tripwire_NoCycleInPath_FailOpen(t *testing.T) {
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 0, durSecs: 90, cycleInDir: false,
	})

	line, ok := tripwireStderrLine(stderr)
	if !ok {
		t.Fatalf("tripwire must still fire when the cycle is not derivable (fail-open); stderr:\n%s", stderr)
	}
	for _, needle := range []string{"agy", "builder"} {
		if !strings.Contains(line, needle) {
			t.Errorf("fail-open TRIPWIRE line must still name %q, got: %s", needle, line)
		}
	}
}

func TestRecordTokenUsage_TripwireRecord_FieldFlips(t *testing.T) {
	_, tripped := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 0, durSecs: 90, cycleInDir: true,
	})
	if !strings.Contains(tripped, `"tripwire":true`) {
		t.Errorf("escalating launch record must carry \"tripwire\":true, got: %s", tripped)
	}

	_, quiet := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 85, durSecs: 5, cycleInDir: true,
	})
	if !strings.Contains(quiet, `"tripwire":false`) {
		t.Errorf("non-escalating launch record must carry \"tripwire\":false (present, queryable), got: %s", quiet)
	}
}
