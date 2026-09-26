package phaseobserver

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// babbleStream returns n assistant-text lines with no tool use, in the stream-json shape claude -p emits.
func babbleStream(n int) []string {
	lines := make([]string, 0, n)
	for i := 0; i < n; i++ {
		lines = append(lines, `{"type":"assistant","message":{"content":[{"type":"text","text":"more tokens"}]}}`)
	}
	return lines
}

func toolUseStream() string {
	return `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file":"x.go"}}]}}`
}

func TestRun_MaxNoProgress_BabbleAgent_Fires(t *testing.T) {
	t.Parallel()
	ws := tempWorkspace(t)
	stdoutPath := filepath.Join(ws, "builder-stdout.log")
	if err := os.WriteFile(stdoutPath, []byte(strings.Join(babbleStream(20), "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("seed log: %v", err)
	}

	killCalls := 0
	mu := &sync.Mutex{}
	startTime := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	callIdx := 0
	// Reads after the sixth jump 700 s: past MaxNoProgressS, while StallS 9999 keeps the idle rule quiet.
	nowFn := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		callIdx++
		if callIdx <= 6 {
			return startTime
		}
		return startTime.Add(700 * time.Second)
	}
	killFn := func(pgid int, sig syscall.Signal) error {
		mu.Lock()
		killCalls++
		mu.Unlock()
		return nil
	}

	rc := Run(Config{
		Workspace: ws, SubagentPGID: 12345, Cycle: 1,
		Phase: "build", Agent: "builder",
		PollS:          1,
		StallS:         9999,
		MaxNoProgressS: 600,
		EOFGraceS:      9999,
		Enforce:        true,
		Now:            nowFn,
		KillPgrp:       killFn,
		StopAfterMS:    800,
	}, stdoutPath, os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}

	mu.Lock()
	defer mu.Unlock()
	if killCalls == 0 {
		t.Error("expected at least 1 kill call when ENFORCE + no-progress trips, got 0")
	}
	events, err := os.ReadFile(filepath.Join(ws, "builder-observer-events.ndjson"))
	if err != nil {
		t.Fatalf("events file missing: %v", err)
	}
	if !strings.Contains(string(events), "stuck_no_progress") {
		t.Errorf("missing stuck_no_progress INCIDENT:\n%s", events)
	}
	if strings.Contains(string(events), "stuck_no_output") {
		t.Errorf("idle StallS=9999 should NOT have fired:\n%s", events)
	}
}

func TestRun_MaxNoProgress_ToolUsingAgent_DoesNotFire(t *testing.T) {
	t.Parallel()
	ws := tempWorkspace(t)
	stdoutPath := filepath.Join(ws, "builder-stdout.log")
	lines := []string{toolUseStream(), toolUseStream(), toolUseStream()}
	if err := os.WriteFile(stdoutPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("seed log: %v", err)
	}

	killCalls := 0
	mu := &sync.Mutex{}
	startTime := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	callIdx := 0
	// The clock never leaves the MaxNoProgressS window, so this guards false positives only;
	// the babble test is the one that proves which events count as progress.
	nowFn := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		callIdx++
		return startTime.Add(time.Duration(callIdx) * 10 * time.Second)
	}
	killFn := func(pgid int, sig syscall.Signal) error {
		mu.Lock()
		killCalls++
		mu.Unlock()
		return nil
	}

	rc := Run(Config{
		Workspace: ws, SubagentPGID: 12345, Cycle: 1,
		Phase: "build", Agent: "builder",
		PollS:          1,
		StallS:         9999,
		MaxNoProgressS: 600,
		EOFGraceS:      9999,
		Enforce:        true,
		Now:            nowFn,
		KillPgrp:       killFn,
		StopAfterMS:    400,
	}, stdoutPath, os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}

	mu.Lock()
	defer mu.Unlock()
	if killCalls != 0 {
		t.Errorf("tool-using agent killed unexpectedly (%d times)", killCalls)
	}
	events, err := os.ReadFile(filepath.Join(ws, "builder-observer-events.ndjson"))
	if err == nil {
		if strings.Contains(string(events), "stuck_no_progress") {
			t.Errorf("stuck_no_progress fired on a healthy tool-using agent:\n%s", events)
		}
	}
}

func TestRun_MaxNoProgress_Disabled_IsLegacyByteIdentical(t *testing.T) {
	t.Parallel()
	ws := tempWorkspace(t)
	stdoutPath := filepath.Join(ws, "builder-stdout.log")
	if err := os.WriteFile(stdoutPath, []byte(strings.Join(babbleStream(20), "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("seed log: %v", err)
	}

	mu := &sync.Mutex{}
	startTime := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	callIdx := 0
	nowFn := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		callIdx++
		if callIdx <= 6 {
			return startTime
		}
		return startTime.Add(700 * time.Second)
	}

	rc := Run(Config{
		Workspace: ws, SubagentPGID: 12345, Cycle: 1,
		Phase: "build", Agent: "builder",
		PollS:          1,
		StallS:         9999,
		MaxNoProgressS: 0,
		EOFGraceS:      9999,
		Now:            nowFn,
		StopAfterMS:    400,
	}, stdoutPath, os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	events, _ := os.ReadFile(filepath.Join(ws, "builder-observer-events.ndjson"))
	if strings.Contains(string(events), "stuck_no_progress") {
		t.Errorf("MaxNoProgressS=0 must NOT emit stuck_no_progress:\n%s", events)
	}
}
