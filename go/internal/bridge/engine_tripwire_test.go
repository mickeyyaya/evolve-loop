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
	cli        string
	agent      string
	code       int
	durSecs    int
	covered    bool
	cycleInDir bool
}

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
		envelope := `{"kind":"result","data":{"cost_usd":0.4,"tokens":{"in":900,"out":210,"cache_r":30,"cache_c":7}}}` + "\n"
		if err := os.WriteFile(filepath.Join(ws, c.agent+"-events.ndjson"), []byte(envelope), 0o644); err != nil {
			t.Fatalf("write events fixture: %v", err)
		}
	}

	var errBuf bytes.Buffer
	signals := signalcenter.New()
	signals.Subscribe(signalcenter.Filter(signalcenter.StderrSink(&errBuf), signalcenter.SeverityWarn))
	e := NewEngine(Deps{
		Now:           func() time.Time { return end },
		Stderr:        io.Discard,
		Signals:       signals,
		TokenResolver: tokenusage.DefaultResolver(t.TempDir()),
	})
	req := core.BridgeRequest{
		CLI:       c.cli,
		Agent:     c.agent,
		Workspace: ws,
		Worktree:  "/repo/worktrees/cycle-1005",
	}
	if c.cycleInDir {
		req.Cycle = 1005
	}
	var resp core.BridgeResponse
	e.recordTokenUsage(req, "sonnet", c.code, start, &resp)
	return errBuf.String(), readLLMCalls(t, ws)
}

func tripwireStderrLine(stderr string) (string, bool) {
	for _, ln := range strings.Split(stderr, "\n") {
		if strings.Contains(ln, "TRIPWIRE") {
			return ln, true
		}
	}
	return "", false
}

func TestRecordTokenUsage_Tripwire_NonClaudeExit0Success_Warns(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 85, durSecs: 5, cycleInDir: true,
	})

	if line, ok := tripwireStderrLine(stderr); ok {
		t.Errorf("exit-85 short quota-abort must NOT trip the tripwire (false positive), got: %s", line)
	}
}

func TestRecordTokenUsage_Tripwire_Exit0ShortDuration_Silent(t *testing.T) {
	t.Parallel()
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 0, durSecs: 5, cycleInDir: true,
	})

	if line, ok := tripwireStderrLine(stderr); ok {
		t.Errorf("exit-0 but sub-threshold duration must NOT trip the tripwire, got: %s", line)
	}
}

func TestRecordTokenUsage_Tripwire_AClaudeLaunchWhoseCollectorFoundNothingWarns(t *testing.T) {
	t.Parallel()
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "claude-tmux", agent: "builder", code: 0, durSecs: 90, cycleInDir: true,
	})

	if _, ok := tripwireStderrLine(stderr); !ok {
		t.Errorf("claude has its own collector (the transcript), so a long successful launch with no usage must trip; stderr:\n%s", stderr)
	}
}

func TestRecordTokenUsage_Tripwire_NoCollectorFamily_Silent(t *testing.T) {
	t.Parallel()
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "codex-tmux", agent: "builder", code: 0, durSecs: 90, cycleInDir: true,
	})

	if line, ok := tripwireStderrLine(stderr); ok {
		t.Errorf("codex has no usage collector, so its unmeasured launch is a known gap, not a tripwire; got: %s", line)
	}
	if !strings.Contains(stderr, "has no usage collector") {
		t.Errorf("the coverage warning must say why codex is unmeasured; stderr:\n%s", stderr)
	}
}

func TestRecordTokenUsage_Tripwire_CoveredNonClaude_Silent(t *testing.T) {
	t.Parallel()
	stderr, record := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 0, durSecs: 90, covered: true, cycleInDir: true,
	})

	if !strings.Contains(record, `"source":"events_result"`) {
		t.Fatalf("fixture invalid: expected a covered (events_result) resolve, got record: %s", record)
	}
	if line, ok := tripwireStderrLine(stderr); ok {
		t.Errorf("a measured (source=events_result) non-claude success must NOT trip the tripwire, got: %s", line)
	}
}

func TestRecordTokenUsage_Tripwire_NoCycleInPath_FailOpen(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
