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

	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func tempWorkspace(t *testing.T) string {
	t.Helper()
	return fixtures.NewWorkspace(t).Build().Root
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

	// <-done is the real barrier: Run catches an already-closed shutdown on its
	// first select iteration and writes the report before returning. No sleep.
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

func TestRun_TailsStdoutLog(t *testing.T) {
	t.Parallel()
	ws := tempWorkspace(t)
	stdoutLog := filepath.Join(ws, "builder-stdout.log")
	// Pre-seed with 2 events.
	_ = os.WriteFile(stdoutLog, []byte(
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"path":"x"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","is_error":false}]}}
`), 0o644)

	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	// StopAfterMS drives a deterministic termination: Run blocks until the stop
	// timer (200ms), having tailed the pre-seeded stdout on its poll ticks
	// (interval = StopAfterMS/4 = 50ms). No shutdown goroutine racing a sleep.
	rc := Run(Config{
		Workspace: ws, Cycle: 1, Phase: "build", Agent: "builder",
		PollS: 1, StallS: 9999, EOFGraceS: 9999,
		Now:         func() time.Time { return now },
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
