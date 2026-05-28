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

// Workstream E1 tests for the babbling-agent backstop. The pre-E1 idle
// stall (StallS) resets on ANY valid JSON line — including pure
// assistant_text token streaming — so a livelocked agent that emits forever
// never trips it. MaxNoProgressS only resets on tool_use / tool_result, so
// a babbling agent that's doing no real work DOES trip the new INCIDENT.

// babbleStream returns N stream-json lines of pure assistant_text with no
// tool use. Mirrors the live stream-json shape claude -p emits.
func babbleStream(n int) []string {
	lines := make([]string, 0, n)
	for i := 0; i < n; i++ {
		lines = append(lines, `{"type":"assistant","message":{"content":[{"type":"text","text":"more tokens"}]}}`)
	}
	return lines
}

// toolUseStream returns ONE stream-json line of a tool_use event (meaningful
// progress).
func toolUseStream() string {
	return `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file":"x.go"}}]}}`
}

// TestRun_MaxNoProgress_BabbleAgent_Fires is the E1 invariant: an agent
// streaming pure assistant_text with NO tool_use trips stuck_no_progress
// despite never triggering the idle StallS rule.
func TestRun_MaxNoProgress_BabbleAgent_Fires(t *testing.T) {
	t.Parallel()
	ws := tempWorkspace(t)
	stdoutPath := filepath.Join(ws, "builder-stdout.log")
	// Seed the log with babbling text — these resets lastEventTS (idle clock
	// stays low) but NOT lastProgressTS (progress clock keeps ticking).
	if err := os.WriteFile(stdoutPath, []byte(strings.Join(babbleStream(20), "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("seed log: %v", err)
	}

	killCalls := 0
	mu := &sync.Mutex{}
	startTime := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	callIdx := 0
	// First few Now() calls are at startTime (init + emit + first poll see
	// the babbling); subsequent calls jump past MaxNoProgressS so the
	// progress check fires while the idle check would NOT (lastEventTS was
	// just bumped by the babble lines).
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
		StallS:         9999, // idle stall would NEVER fire (babble keeps lastEventTS fresh)
		MaxNoProgressS: 600,  // but no-progress catches the livelock
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
	// And the idle stall must NOT have fired (assistant_text keeps
	// lastEventTS fresh; we only set MaxNoProgressS to trip).
	if strings.Contains(string(events), "stuck_no_output") {
		t.Errorf("idle StallS=9999 should NOT have fired:\n%s", events)
	}
}

// TestRun_MaxNoProgress_ToolUsingAgent_DoesNotFire proves E1 is correctly
// scoped: an agent doing real work (tool_use events) bumps lastProgressTS
// on each tool dispatch, so the no-progress clock stays low and no INCIDENT
// fires. This is the false-positive guard.
func TestRun_MaxNoProgress_ToolUsingAgent_DoesNotFire(t *testing.T) {
	t.Parallel()
	ws := tempWorkspace(t)
	stdoutPath := filepath.Join(ws, "builder-stdout.log")
	// Seed with several tool_use events — each resets lastProgressTS.
	lines := []string{toolUseStream(), toolUseStream(), toolUseStream()}
	if err := os.WriteFile(stdoutPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("seed log: %v", err)
	}

	killCalls := 0
	mu := &sync.Mutex{}
	startTime := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	callIdx := 0
	// Now() advances only modestly — within the MaxNoProgressS window — so a
	// healthy tool-using agent never trips. (If lastProgressTS were buggy
	// and reset on every line like lastEventTS, this would still not fire.
	// The real test of the SCOPING is the babble-agent test above.)
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

// TestRun_MaxNoProgress_Disabled_IsLegacyByteIdentical pins the opt-in
// contract: when MaxNoProgressS==0 (default), the feature is off — no
// stuck_no_progress emit even when the clock would otherwise trip. Pre-E1
// posture is preserved for operators who don't opt in.
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
		MaxNoProgressS: 0, // OFF — legacy posture
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
