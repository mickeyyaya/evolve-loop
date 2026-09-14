package phaseobserver

// order_pins_test.go — ADR-0103 unit 12 step 0: the poll-tick order and the
// rule invariants of Run, pinned on the pre-extraction code (8e8f080f) BEFORE
// the tick body moved into internal/observerengine. Each test names the
// one-line mutant it kills; every one was proven red against that mutant by
// hand before the move. They drive the kept Run facade, so they stay in the
// host as the regression net after the move.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// steppedClock returns t0 for the first `hold` calls and t0+jump afterwards —
// the count-stepping idiom the host suite already uses (coverage_test.go).
func steppedClock(t0 time.Time, hold int, jump time.Duration) func() time.Time {
	var mu sync.Mutex
	calls := 0
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls <= hold {
			return t0
		}
		return t0.Add(jump)
	}
}

// killRecorder counts KillPgrp calls under a mutex (Run's goroutine vs the test's).
type killRecorder struct {
	mu    sync.Mutex
	calls int
	seen  []string
}

func (k *killRecorder) fn(int, syscall.Signal) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.calls++
	return nil
}

func (k *killRecorder) count() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.calls
}

func seedLog(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func eventsOf(t *testing.T, ws string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(ws, "builder-observer-events.ndjson"))
	if err != nil {
		t.Fatalf("events file: %v", err)
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var env map[string]any
		if err := json.Unmarshal([]byte(line), &env); err != nil {
			t.Fatalf("bad envelope %q: %v", line, err)
		}
		out = append(out, env)
	}
	return out
}

func firstIndexOfType(events []map[string]any, typ string) int {
	for i, e := range events {
		if e["type"] == typ {
			return i
		}
	}
	return -1
}

var threeLines = []string{
	`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"path":"x"}}]}}`,
	`{"type":"user","message":{"content":[{"type":"tool_result","is_error":false}]}}`,
	`{"type":"result","total_cost_usd":0.5,"usage":{"cache_read_input_tokens":10,"cache_creation_input_tokens":2}}`,
}

// TestRun_ProbeRunsBeforeTail — the process probe fires BEFORE the log tail
// (phaseobserver.go:325-338 before :339): with three lines pending, the
// process_dead envelope's id ends `_0` (eventCount before this tick's ingest).
// Kills M1 (ingest hoisted above the probe → `_3`).
func TestRun_ProbeRunsBeforeTail(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	seedLog(t, filepath.Join(ws, "builder-stdout.log"), threeLines...)
	kills := &killRecorder{}
	rc := Run(Config{
		Workspace: ws, SubagentPGID: 4242, Cycle: 1, Phase: "build", Agent: "builder",
		PollS: 1, StallS: 9999, EOFGraceS: 9999,
		StallPolicy:  recovery.NewChainStallPolicy(6),
		ProcessAlive: func(int) bool { return false },
		KillPgrp:     kills.fn,
		Now:          fixedClock(goldenAt),
		StopAfterMS:  200,
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	events := eventsOf(t, ws)
	i := firstIndexOfType(events, "process_dead")
	if i < 0 {
		t.Fatalf("no process_dead envelope:\n%v", events)
	}
	if id, _ := events[i]["id"].(string); !strings.HasSuffix(id, "_0") {
		t.Errorf("process_dead id = %q, want the `_0` suffix (probe before ingest)", id)
	}
	if kills.count() != 1 {
		t.Errorf("chain policy → kill_retry exactly once; kills=%d", kills.count())
	}
}

// TestRun_IngestRunsBeforeStallRules — the tick ingests new lines BEFORE the
// stall rules read the idle clock (:339-348 before :350): lines seeded, the
// clock jumps +700 s from the third call on, so the per-line Now stamps refresh
// lastEventTS and the first rule pass sees idle 0. Kills M2 (rules hoisted
// above ingest → stuck_no_output + a kill on the first tick).
func TestRun_IngestRunsBeforeStallRules(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	seedLog(t, filepath.Join(ws, "builder-stdout.log"), threeLines...)
	kills := &killRecorder{}
	rc := Run(Config{
		Workspace: ws, SubagentPGID: 99999, Cycle: 1, Phase: "build", Agent: "builder",
		PollS: 1, StallS: 600, EOFGraceS: 9999, Enforce: true,
		Now:         steppedClock(goldenAt, 2, 700*time.Second),
		KillPgrp:    kills.fn,
		StopAfterMS: 200,
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	if kills.count() != 0 {
		t.Errorf("no kill when the lines refresh the idle clock first; kills=%d", kills.count())
	}
	if i := firstIndexOfType(eventsOf(t, ws), "stuck_no_output"); i >= 0 {
		t.Errorf("stuck_no_output must not fire on a tick that just ingested lines")
	}
}

// TestRun_NudgeThenHardStallInOneTick — inside one tick the soft-stall nudge
// (:356-369) runs BEFORE the hard stall (:370-379): with idle 700 ≥ both
// thresholds the soft_stall_nudge envelope sits on an EARLIER line than the
// first stuck_no_output, and the inbox holds exactly one nudge. Kills M3 (the
// two rules swapped) and M4 (`!obs.nudged` dropped → a nudge per tick).
func TestRun_NudgeThenHardStallInOneTick(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	kills := &killRecorder{}
	rc := Run(Config{
		Workspace: ws, SubagentPGID: 99999, Cycle: 1, Phase: "build", Agent: "builder",
		PollS: 1, StallS: 600, NudgeS: 300, EOFGraceS: 9999, Enforce: true,
		Now:         steppedClock(goldenAt, 2, 700*time.Second),
		KillPgrp:    kills.fn,
		StopAfterMS: 300,
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	events := eventsOf(t, ws)
	nudge, stall := firstIndexOfType(events, "soft_stall_nudge"), firstIndexOfType(events, "stuck_no_output")
	if nudge < 0 || stall < 0 || nudge > stall {
		t.Errorf("soft_stall_nudge (line %d) must precede stuck_no_output (line %d)", nudge, stall)
	}
	raw, err := os.ReadFile(filepath.Join(ws, ".bridge-inbox", "builder.ndjson"))
	if err != nil {
		t.Fatalf("inbox: %v", err)
	}
	if n := strings.Count(string(raw), `"nudge"`); n != 1 {
		t.Errorf("exactly one nudge envelope over several stalled ticks (nudged dedupes), got %d:\n%s", n, raw)
	}
	if kills.count() == 0 {
		t.Error("Enforce + hard stall must kill")
	}
}

// TestRun_HeartbeatCountsThisTicksLines — the heartbeat (:407-415) runs AFTER
// ingest: with HeartbeatEvery 1 and four lines seeded, the first heartbeat's
// event_count is already 4. Kills M10 (heartbeat hoisted above ingest → 0).
func TestRun_HeartbeatCountsThisTicksLines(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	seedLog(t, filepath.Join(ws, "builder-stdout.log"), append(threeLines, `{"type":"rate_limit_event"}`)...)
	rc := Run(Config{
		Workspace: ws, Cycle: 1, Phase: "build", Agent: "builder",
		PollS: 1, StallS: 9999, EOFGraceS: 9999, HeartbeatEvery: 1,
		Now:         fixedClock(goldenAt),
		StopAfterMS: 200,
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	events := eventsOf(t, ws)
	i := firstIndexOfType(events, "heartbeat")
	if i < 0 {
		t.Fatalf("no heartbeat:\n%v", events)
	}
	data, _ := events[i]["data"].(map[string]any)
	if n, _ := data["event_count"].(float64); int(n) != 4 {
		t.Errorf("first heartbeat event_count = %v, want 4 (heartbeat after ingest)", data["event_count"])
	}
}

// TestRun_EOFGraceNeedsAtLeastOneEvent — the EOF-grace shutdown (:418) needs
// eventCount > 0: an empty log never self-terminates, the stop timer ends the
// run. Kills M11 (`obs.eventCount > 0` dropped → reason eof_grace).
func TestRun_EOFGraceNeedsAtLeastOneEvent(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	rc := Run(Config{
		Workspace: ws, Cycle: 1, Phase: "build", Agent: "builder",
		PollS: 1, StallS: 9999, EOFGraceS: 1,
		Now:         fixedClock(goldenAt),
		StopAfterMS: 300,
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	events := eventsOf(t, ws)
	i := firstIndexOfType(events, "observer_shutdown")
	if i < 0 {
		t.Fatalf("no observer_shutdown:\n%v", events)
	}
	data, _ := events[i]["data"].(map[string]any)
	if data["reason"] != "stop-timer" {
		t.Errorf("shutdown reason = %v, want stop-timer (eof_grace needs at least one event)", data["reason"])
	}
}

// TestRun_IncidentIsAppendedBeforeTheKill — the INCIDENT envelope is on disk
// BEFORE SIGTERM is sent (:272→:275 and :290→:293): the KillPgrp closure reads
// the events file at kill time and finds stuck_no_output. Kills M9 (kill
// hoisted above the emit).
func TestRun_IncidentIsAppendedBeforeTheKill(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	var mu sync.Mutex
	var seenAtKill []bool
	rc := Run(Config{
		Workspace: ws, SubagentPGID: 99999, Cycle: 1, Phase: "build", Agent: "builder",
		PollS: 1, StallS: 600, EOFGraceS: 9999, Enforce: true,
		Now: steppedClock(goldenAt, 2, 1000*time.Second),
		KillPgrp: func(int, syscall.Signal) error {
			raw, _ := os.ReadFile(filepath.Join(ws, "builder-observer-events.ndjson"))
			mu.Lock()
			seenAtKill = append(seenAtKill, strings.Contains(string(raw), `"stuck_no_output"`))
			mu.Unlock()
			return nil
		},
		StopAfterMS: 200,
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seenAtKill) == 0 {
		t.Fatal("expected at least one kill")
	}
	if !seenAtKill[0] {
		t.Error("the first kill must find its INCIDENT already appended (emit before kill)")
	}
}

// TestRun_KillRetryWithoutPGID_RecordsSkippedAndNeverKills — the
// record-reflects-reality invariant (:280-287): a kill_retry verdict with no
// pgid records `kill_retry_skipped_no_pgid` and never calls KillPgrp. Kills M6
// (`effective = string(action)` unconditionally) and M6b (`willKill` without
// the pgid guard).
func TestRun_KillRetryWithoutPGID_RecordsSkippedAndNeverKills(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	kills := &killRecorder{}
	rc := Run(Config{
		Workspace: ws, SubagentPGID: 0, Cycle: 1, Phase: "build", Agent: "builder",
		PollS: 1, StallS: 600, EOFGraceS: 9999, Enforce: true,
		StallPolicy: &scriptedStallPolicy{action: recovery.StallKillRetry, reason: "dead pane"},
		Now:         steppedClock(goldenAt, 2, 1000*time.Second),
		KillPgrp:    kills.fn,
		StopAfterMS: 200,
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	raw, _ := os.ReadFile(filepath.Join(ws, "builder-observer-events.ndjson"))
	if !strings.Contains(string(raw), `"action":"kill_retry_skipped_no_pgid"`) {
		t.Errorf("the envelope must record the skipped kill:\n%s", raw)
	}
	if kills.count() != 0 {
		t.Errorf("no pgid → no kill; got %d", kills.count())
	}
}

// TestRun_ValidationLinesAreVerbatim — the three validation lines (:176/:180/
// :184) STAY on the injected stderr, byte-for-byte, with ExitInvalidArgs; any
// rewording is red.
func TestRun_ValidationLinesAreVerbatim(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{"usage", Config{Phase: "build", Agent: "builder", Cycle: 1}, "[phase-observer] usage: phase-observer <workspace> <pgid> <cycle> <phase> <agent> [cycle-state]\n"},
		{"workspace", Config{Workspace: filepath.Join(ws, "missing"), Phase: "build", Agent: "builder", Cycle: 1}, "[phase-observer] workspace not a directory: " + filepath.Join(ws, "missing") + "\n"},
		{"cycle", Config{Workspace: ws, Phase: "build", Agent: "builder", Cycle: 0}, "[phase-observer] cycle must be integer\n"},
	}
	for _, tc := range cases {
		var b bytes.Buffer
		if rc := Run(tc.cfg, "", &b); rc != ExitInvalidArgs {
			t.Errorf("%s: rc=%d, want %d", tc.name, rc, ExitInvalidArgs)
		}
		if b.String() != tc.want {
			t.Errorf("%s: stderr = %q, want %q", tc.name, b.String(), tc.want)
		}
	}
}

// TestRun_UnknownScopeRunsNoRules — characterization of the :351 tautology: a
// Scope that is neither phase nor cycle runs NO stall rule (no incident, no
// kill) while observer_started still records it verbatim. The :37 comment's
// "cycle-scope runs only stall_no_output" was never implemented; any third
// value disables ALL rules. Kills M12 (`==` → `!=` on the scope check).
func TestRun_UnknownScopeRunsNoRules(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	kills := &killRecorder{}
	rc := Run(Config{
		Workspace: ws, SubagentPGID: 99999, Cycle: 1, Phase: "build", Agent: "builder",
		Scope: Scope("batch"), PollS: 1, StallS: 600, EOFGraceS: 9999, Enforce: true,
		Now:         steppedClock(goldenAt, 2, 1000*time.Second),
		KillPgrp:    kills.fn,
		StopAfterMS: 200,
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	events := eventsOf(t, ws)
	if firstIndexOfType(events, "stuck_no_output") >= 0 || kills.count() != 0 {
		t.Errorf("an unknown scope runs no rules; kills=%d events=%v", kills.count(), events)
	}
	data, _ := events[0]["data"].(map[string]any)
	if events[0]["type"] != "observer_started" || data["scope"] != "batch" {
		t.Errorf("observer_started records scope=batch verbatim: %v", events[0])
	}
}
