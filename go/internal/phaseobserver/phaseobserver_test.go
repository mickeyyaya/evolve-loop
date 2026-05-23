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
)

func tempWorkspace(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func TestRun_RejectsBadInputs(t *testing.T) {
	t.Parallel()
	ws := tempWorkspace(t)
	tests := []struct {
		name string
		cfg  Config
	}{
		{"no-workspace", Config{Phase: "build", Agent: "builder", Cycle: 1}},
		{"no-phase", Config{Workspace: ws, Agent: "builder", Cycle: 1}},
		{"no-agent", Config{Workspace: ws, Phase: "build", Cycle: 1}},
		{"zero-cycle", Config{Workspace: ws, Phase: "build", Agent: "builder", Cycle: 0}},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var b bytes.Buffer
			if rc := Run(tc.cfg, "", &b); rc != ExitInvalidArgs {
				t.Errorf("rc=%d, want %d (log=%s)", rc, ExitInvalidArgs, b.String())
			}
		})
	}
}

func TestRun_BadWorkspacePath(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	rc := Run(Config{
		Workspace: "/nonexistent-xyz", Phase: "build", Agent: "builder", Cycle: 1,
	}, "", &b)
	if rc != ExitInvalidArgs {
		t.Errorf("rc=%d, want %d", rc, ExitInvalidArgs)
	}
}

func TestRun_WritesShutdownReport(t *testing.T) {
	t.Parallel()
	ws := tempWorkspace(t)
	shutdown := make(chan struct{})
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)

	var rc int
	done := make(chan struct{})
	go func() {
		rc = Run(Config{
			Workspace: ws, Cycle: 1, Phase: "build", Agent: "builder",
			PollS: 1, StallS: 600, EOFGraceS: 100,
			Now:         func() time.Time { return now },
			ShutdownSig: shutdown,
		}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	close(shutdown)
	<-done

	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	report, err := os.ReadFile(filepath.Join(ws, "builder-observer-report.json"))
	if err != nil {
		t.Fatalf("report missing: %v", err)
	}
	var doc map[string]any
	_ = json.Unmarshal(report, &doc)
	if doc["agent"] != "builder" || doc["phase"] != "build" {
		t.Errorf("report fields wrong: %v", doc)
	}
}

func TestProcessLine_AssistantToolUse(t *testing.T) {
	t.Parallel()
	o := &Observer{cfg: Config{Now: time.Now}}
	o.processLine(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"ls"}}]}}`)
	if o.toolCallCount != 1 {
		t.Errorf("tool_call_count = %d, want 1", o.toolCallCount)
	}
	if o.eventCount != 1 {
		t.Errorf("event_count = %d, want 1", o.eventCount)
	}
	if len(o.loopHistory) != 1 {
		t.Errorf("loop history not appended")
	}
	if o.loopHistory[0].tool != "Bash" {
		t.Errorf("tool name = %q, want Bash", o.loopHistory[0].tool)
	}
}

func TestProcessLine_UserToolResultError(t *testing.T) {
	t.Parallel()
	o := &Observer{cfg: Config{Now: time.Now}}
	o.processLine(`{"type":"user","message":{"content":[{"type":"tool_result","is_error":true}]}}`)
	if o.toolResultCnt != 1 || o.errorCount != 1 {
		t.Errorf("expected tool_result and error increments; got tr=%d err=%d",
			o.toolResultCnt, o.errorCount)
	}
}

func TestProcessLine_ResultEventAccumulatesCost(t *testing.T) {
	t.Parallel()
	o := &Observer{cfg: Config{Now: time.Now}}
	o.processLine(`{"type":"result","total_cost_usd":0.45,"usage":{"cache_read_input_tokens":1024,"cache_creation_input_tokens":256}}`)
	if o.cumulativeCost != 0.45 {
		t.Errorf("cost = %v, want 0.45", o.cumulativeCost)
	}
	if o.cacheReadTok != 1024 || o.cacheCreateTok != 256 {
		t.Errorf("cache tokens wrong: r=%d c=%d", o.cacheReadTok, o.cacheCreateTok)
	}
}

func TestProcessLine_RateLimitEventTracked(t *testing.T) {
	t.Parallel()
	o := &Observer{cfg: Config{Now: time.Now}}
	o.processLine(`{"type":"rate_limit_event","reason":"quota"}`)
	if o.rateLimitCnt != 1 {
		t.Errorf("rate_limit_count = %d, want 1", o.rateLimitCnt)
	}
	if len(o.rateLimitHist) != 1 {
		t.Error("rate limit history not appended")
	}
}

func TestProcessLine_MalformedJSONSkipped(t *testing.T) {
	t.Parallel()
	o := &Observer{cfg: Config{Now: time.Now}}
	o.processLine("not json at all")
	o.processLine("")
	if o.eventCount != 0 {
		t.Errorf("malformed/empty should not count, got %d", o.eventCount)
	}
}

func TestRun_StallDetectionFires(t *testing.T) {
	t.Parallel()
	ws := tempWorkspace(t)
	killCalls := 0
	mu := &sync.Mutex{}
	startTime := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	callIdx := 0
	nowFn := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		callIdx++
		// First two calls (setup + first emit) at startTime; subsequent at +1000s
		if callIdx <= 2 {
			return startTime
		}
		return startTime.Add(1000 * time.Second)
	}
	killFn := func(pgid int, sig syscall.Signal) error {
		mu.Lock()
		killCalls++
		mu.Unlock()
		return nil
	}

	rc := Run(Config{
		Workspace: ws, SubagentPGID: 99999, Cycle: 1,
		Phase: "build", Agent: "builder",
		PollS: 1, StallS: 600, EOFGraceS: 9999,
		Enforce:     true,
		Now:         nowFn,
		KillPgrp:    killFn,
		StopAfterMS: 800,
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	mu.Lock()
	defer mu.Unlock()
	if killCalls == 0 {
		t.Errorf("expected at least 1 kill call when ENFORCE+stall, got 0")
	}

	events, err := os.ReadFile(filepath.Join(ws, "builder-observer-events.ndjson"))
	if err != nil {
		t.Fatalf("events file missing: %v", err)
	}
	if !strings.Contains(string(events), "stuck_no_output") {
		t.Errorf("missing stuck_no_output event:\n%s", events)
	}
}

func TestRun_NoEnforceMode_NoKillOnStall(t *testing.T) {
	t.Parallel()
	ws := tempWorkspace(t)
	killCalls := 0
	mu := &sync.Mutex{}
	startTime := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	callIdx := 0
	nowFn := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		callIdx++
		if callIdx <= 2 {
			return startTime
		}
		return startTime.Add(1000 * time.Second)
	}
	rc := Run(Config{
		Workspace: ws, SubagentPGID: 99999, Cycle: 1,
		Phase: "build", Agent: "builder",
		PollS: 1, StallS: 600, EOFGraceS: 9999,
		Enforce: false,
		Now:     nowFn,
		KillPgrp: func(int, syscall.Signal) error {
			mu.Lock()
			killCalls++
			mu.Unlock()
			return nil
		},
		StopAfterMS: 800,
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	if killCalls > 0 {
		t.Errorf("non-enforce mode should not kill; got %d calls", killCalls)
	}
}

func TestEmit_AppendsToEventsFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	o := &Observer{
		cfg:          Config{Now: time.Now, Cycle: 5, Phase: "audit", Agent: "auditor"},
		traceID:      "trace-x",
		lastEventTS:  time.Now(),
	}
	o.cfg.Now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	events := filepath.Join(dir, "events.ndjson")
	o.emit(events, "test_event", "INCIDENT", map[string]any{"foo": "bar"})
	body, err := os.ReadFile(events)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, body)
	}
	if doc["type"] != "test_event" {
		t.Errorf("type = %v", doc["type"])
	}
	if doc["severity"] != "INCIDENT" {
		t.Errorf("severity = %v", doc["severity"])
	}
	if len(o.incidents) != 1 {
		t.Errorf("INCIDENT should be tracked, got %d", len(o.incidents))
	}
}

func TestRun_TailsStdoutLog(t *testing.T) {
	t.Parallel()
	ws := tempWorkspace(t)
	stdoutLog := filepath.Join(ws, "builder-stdout.log")
	// Pre-seed with 2 events.
	_ = os.WriteFile(stdoutLog, []byte(
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"path":"x"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","is_error":false}]}}
`), 0o644)

	shutdown := make(chan struct{})
	go func() {
		time.Sleep(150 * time.Millisecond)
		close(shutdown)
	}()
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	rc := Run(Config{
		Workspace: ws, Cycle: 1, Phase: "build", Agent: "builder",
		PollS: 1, StallS: 9999, EOFGraceS: 9999,
		Now:         func() time.Time { return now },
		ShutdownSig: shutdown,
		StopAfterMS: 200,
	}, stdoutLog, os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	report, _ := os.ReadFile(filepath.Join(ws, "builder-observer-report.json"))
	var doc map[string]any
	_ = json.Unmarshal(report, &doc)
	tc, _ := doc["tool_call_count"].(float64)
	if int(tc) != 1 {
		t.Errorf("tool_call_count = %v, want 1\n%s", tc, report)
	}
	tr, _ := doc["tool_result_count"].(float64)
	if int(tr) != 1 {
		t.Errorf("tool_result_count = %v, want 1", tr)
	}
}
