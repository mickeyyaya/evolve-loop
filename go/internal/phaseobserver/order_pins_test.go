package phaseobserver

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

// steppedClock returns t0 for the first hold calls and t0+jump afterwards.
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

// killRecorder counts KillPgrp calls; the mutex guards Run's goroutine against the test's.
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
