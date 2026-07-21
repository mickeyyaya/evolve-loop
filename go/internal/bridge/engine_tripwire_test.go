package bridge

// engine_tripwire_test.go — RED contract for cycle-1005, fleet item
// telemetry-coverage-tripwire-nonclaude-success (scout Task 1
// telemetry-tripwire-nonclaude-exit0-warn + Task 2
// telemetry-tripwire-llm-calls-record).
//
// Today recordTokenUsage (engine.go:597-600) fires ONE generic per-driver
// coverage WARN on every SourceNone resolution, regardless of exit code or
// duration. So a quiet quota-abort (exit 85, a few seconds) and a genuine
// unmeasured success (exit 0, minutes long, non-claude) read IDENTICALLY — the
// operator cannot tell "nothing to see" from "go build a collector now".
//
// This contract requires an ESCALATION on top of the existing generic WARN: when
// a non-claude launch exits 0, ran longer than the success threshold (>60s), and
// still resolved to source=none, recordTokenUsage must emit a distinct
// TRIPWIRE-marked stderr line naming the CLI, the agent, and (best-effort, from
// the workspace path) the cycle — and fold a queryable "tripwire":true field into
// the llm-calls.ndjson record. Quota-abort, claude-baseline, covered, and
// short-duration launches must NOT trip.
//
// DO NOT modify these tests. Make them pass by adding the escalation at the
// call site (resolver stays fail-open, per engine.go:558-566).

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

// tripwireCase parameterizes one recordTokenUsage invocation.
type tripwireCase struct {
	cli        string // driver identity (req.CLI)
	agent      string // req.Agent
	code       int    // launch exit code
	durSecs    int    // wall-clock duration of the launch (end - start)
	covered    bool   // true => a workspace events log makes the resolve source=events_result
	cycleInDir bool   // true => workspace lives under .evolve/runs/cycle-1005
}

// runTripwireCase drives recordTokenUsage for one case and returns the captured
// stderr and the appended llm-calls.ndjson contents. It reuses readLLMCalls from
// tokenfallback_red_test.go (same package).
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
		// eventsResult tier fire, so the resolve is source=events_result (covered),
		// not source=none — nothing unmeasured, so the tripwire must stay silent.
		envelope := `{"kind":"result","data":{"cost_usd":0.4,"tokens":{"in":900,"out":210,"cache_r":30,"cache_c":7}}}` + "\n"
		if err := os.WriteFile(filepath.Join(ws, c.agent+"-events.ndjson"), []byte(envelope), 0o644); err != nil {
			t.Fatalf("write events fixture: %v", err)
		}
	}

	var errBuf bytes.Buffer
	e := NewEngine(Deps{
		Now:           func() time.Time { return end },
		Stderr:        &errBuf,
		TokenResolver: tokenusage.DefaultResolver(t.TempDir()), // empty root: no transcript tier
	})
	req := core.BridgeRequest{
		CLI:       c.cli,
		Agent:     c.agent,
		Workspace: ws,
		Worktree:  "/repo/worktrees/cycle-1005",
	}
	var resp core.BridgeResponse
	e.recordTokenUsage(req, "sonnet", c.code, start, &resp)
	return errBuf.String(), readLLMCalls(t, ws)
}

// tripwireStderrLine returns the first stderr line carrying the TRIPWIRE marker.
// Keying on the marker is essential: the pre-existing generic coverage WARN
// already contains the CLI and agent names, so a substring check for those alone
// would false-green on the generic WARN. Only the TRIPWIRE marker distinguishes
// the escalation.
func tripwireStderrLine(stderr string) (string, bool) {
	for _, ln := range strings.Split(stderr, "\n") {
		if strings.Contains(ln, "TRIPWIRE") {
			return ln, true
		}
	}
	return "", false
}

// TestRecordTokenUsage_Tripwire_NonClaudeExit0Success_Warns — AC1+AC2 (POSITIVE).
// A non-claude launch that exited 0, ran >60s, and resolved to source=none must
// emit a distinct TRIPWIRE stderr line, and that same line must name the CLI, the
// agent, and the cycle (derived from the workspace path).
func TestRecordTokenUsage_Tripwire_NonClaudeExit0Success_Warns(t *testing.T) {
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 0, durSecs: 90, cycleInDir: true,
	})

	line, ok := tripwireStderrLine(stderr)
	if !ok {
		t.Fatalf("expected a TRIPWIRE escalation line for an exit-0 >60s non-claude source=none launch; stderr:\n%s", stderr)
	}
	for _, needle := range []string{"agy", "builder", "cycle-1005"} {
		if !strings.Contains(line, needle) {
			t.Errorf("TRIPWIRE line must name %q (CLI+agent+cycle contract), got: %s", needle, line)
		}
	}
}

// TestRecordTokenUsage_Tripwire_QuotaAbortShortLaunch_Silent — AC3 (NEGATIVE,
// highest-leverage anti-false-positive). The current real traffic pattern: a
// non-claude launch that quota-aborts (exit 85) after a few seconds must NOT trip
// the escalation — only the pre-existing generic coverage WARN may appear.
func TestRecordTokenUsage_Tripwire_QuotaAbortShortLaunch_Silent(t *testing.T) {
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 85, durSecs: 5, cycleInDir: true,
	})

	if line, ok := tripwireStderrLine(stderr); ok {
		t.Errorf("exit-85 short quota-abort must NOT trip the tripwire (false positive), got: %s", line)
	}
}

// TestRecordTokenUsage_Tripwire_Exit0ShortDuration_Silent — NEGATIVE (isolates the
// duration gate). An exit-0 non-claude launch that finished in a few seconds
// (e.g. a boot-smoke or an instant no-op) is not a genuine unmeasured success, so
// it must stay silent even though its exit code is 0.
func TestRecordTokenUsage_Tripwire_Exit0ShortDuration_Silent(t *testing.T) {
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 0, durSecs: 5, cycleInDir: true,
	})

	if line, ok := tripwireStderrLine(stderr); ok {
		t.Errorf("exit-0 but sub-threshold duration must NOT trip the tripwire, got: %s", line)
	}
}

// TestRecordTokenUsage_Tripwire_ClaudeBaseline_Silent — NEGATIVE (SEMANTIC). Claude
// drivers are the measured baseline, not the blind-spot target; a claude launch
// that resolves to source=none (e.g. a retry transcript loss) is a different,
// out-of-scope gap and must NOT trip this non-claude tripwire.
func TestRecordTokenUsage_Tripwire_ClaudeBaseline_Silent(t *testing.T) {
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "claude-tmux", agent: "builder", code: 0, durSecs: 90, cycleInDir: true,
	})

	if line, ok := tripwireStderrLine(stderr); ok {
		t.Errorf("claude is the measured baseline, not the tripwire target; must stay silent, got: %s", line)
	}
}

// TestRecordTokenUsage_Tripwire_CoveredNonClaude_Silent — NEGATIVE (SEMANTIC). The
// tripwire fires on the UNMEASURED signal, not merely on a non-claude success. A
// non-claude launch whose usage was recovered (source=events_result) is measured,
// so no collector is needed and the tripwire must stay silent.
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

// TestRecordTokenUsage_Tripwire_NoCycleInPath_FailOpen — EDGE (fail-open on cycle
// derivation). If the cycle cannot be parsed from the workspace path, the tripwire
// must STILL fire (naming CLI+agent) — the missing cycle is degraded gracefully,
// never an error or a suppressed warning.
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

// TestRecordTokenUsage_TripwireRecord_FieldFlips — Task 2 (AC4, queryability). The
// llm-calls.ndjson record must carry a queryable boolean "tripwire" field: true
// for the escalating exit-0/long/non-claude/source-none launch and false for the
// quota-abort launch, so the future tokens-report CLI can select tripwire records
// without re-parsing stderr. The field must be present (not omitempty) in both.
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
