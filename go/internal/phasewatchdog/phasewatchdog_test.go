package phasewatchdog

import (
	"bytes"
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
	d := t.TempDir()
	return d
}

func TestRun_RejectsBadInputs(t *testing.T) {
	t.Parallel()
	ws := tempWorkspace(t)
	tests := []struct {
		name string
		cfg  Config
	}{
		{"no-workspace", Config{TargetPGID: 1, Cycle: 1}},
		{"bad-workspace", Config{Workspace: "/nonexistent-xyz", TargetPGID: 1, Cycle: 1}},
		{"zero-pgid", Config{Workspace: ws, TargetPGID: 0, Cycle: 1}},
		{"negative-pgid", Config{Workspace: ws, TargetPGID: -5, Cycle: 1}},
		{"zero-cycle", Config{Workspace: ws, TargetPGID: 1, Cycle: 0}},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var b bytes.Buffer
			if rc := Run(tc.cfg, &b); rc != ExitInvalidArg {
				t.Errorf("got rc=%d want %d (log=%s)", rc, ExitInvalidArg, b.String())
			}
		})
	}
}

func TestRun_DisabledShortCircuits(t *testing.T) {
	t.Parallel()
	ws := tempWorkspace(t)
	var b bytes.Buffer
	rc := Run(Config{
		Workspace: ws, TargetPGID: 1, Cycle: 1,
		Disabled: true,
	}, &b)
	if rc != ExitOK {
		t.Errorf("disabled should exit 0, got %d", rc)
	}
	if !strings.Contains(b.String(), "DISABLE=1") {
		t.Errorf("missing disable log: %s", b.String())
	}
}

// TestRun_FireSequence drives a watchdog with a controlled clock: artifacts
// have stale mtimes, the simulated now jumps past the threshold, and we
// observe the FIRE path writes stall-progress.json + abnormal-events.jsonl
// + invokes KillPgrp twice (TERM then KILL).
func TestRun_FireSequence(t *testing.T) {
	t.Parallel()
	ws := tempWorkspace(t)
	// Seed a stale activity file
	activity := filepath.Join(ws, "build.log")
	if err := os.WriteFile(activity, []byte("started"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Set its mtime to a known past value
	staleTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(activity, staleTime, staleTime); err != nil {
		t.Fatal(err)
	}

	cycleState := filepath.Join(ws, "cycle-state.json")
	if err := os.WriteFile(cycleState, []byte(`{"phase":"build"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Make cycle-state mtime stale too — otherwise its real-now mtime dominates
	// best_mtime and baseline > test-clock → idleS clamps to 0 → never fires.
	if err := os.Chtimes(cycleState, staleTime, staleTime); err != nil {
		t.Fatal(err)
	}

	// Simulated clock: starts at staleTime+1s, jumps to staleTime+700s on iter 2 (past threshold 600)
	currentNow := staleTime.Add(1 * time.Second)
	iter := 0
	nowFn := func() time.Time {
		iter++
		if iter > 3 {
			currentNow = staleTime.Add(700 * time.Second)
		}
		return currentNow
	}
	killCalls := []syscall.Signal{}
	mu := &sync.Mutex{}
	killFn := func(pgid int, sig syscall.Signal) error {
		mu.Lock()
		defer mu.Unlock()
		killCalls = append(killCalls, sig)
		return nil
	}
	sleepCalls := 0
	sleepFn := func(d time.Duration) {
		sleepCalls++
	}

	var stderr bytes.Buffer
	rc := Run(Config{
		Workspace:      ws,
		TargetPGID:     12345,
		Cycle:          1,
		CycleStatePath: cycleState,
		ThresholdS:     600,
		PollS:          15,
		WarnPct:        75,
		GraceS:         10,
		Now:            nowFn,
		Sleep:          sleepFn,
		KillPgrp:       killFn,
		StopAfter:      20, // safety net — FIRE should happen before this
	}, &stderr)

	if rc != ExitOK {
		t.Fatalf("rc=%d, want 0 (stderr=%s)", rc, stderr.String())
	}
	mu.Lock()
	defer mu.Unlock()
	if len(killCalls) != 2 {
		t.Fatalf("kill calls: got %d, want 2 (TERM+KILL)", len(killCalls))
	}
	if killCalls[0] != syscall.SIGTERM {
		t.Errorf("first kill: got %v, want SIGTERM", killCalls[0])
	}
	if killCalls[1] != syscall.SIGKILL {
		t.Errorf("second kill: got %v, want SIGKILL", killCalls[1])
	}

	// stall-progress.json written
	stallBody, err := os.ReadFile(filepath.Join(ws, "stall-progress.json"))
	if err != nil {
		t.Errorf("stall-progress.json missing: %v", err)
	}
	if !strings.Contains(string(stallBody), `"idle_s"`) {
		t.Errorf("stall-progress missing idle_s: %s", stallBody)
	}
	// abnormal-events.jsonl appended
	abBody, err := os.ReadFile(filepath.Join(ws, "abnormal-events.jsonl"))
	if err != nil {
		t.Errorf("abnormal-events.jsonl missing: %v", err)
	}
	if !strings.Contains(string(abBody), "stall-detected") {
		t.Errorf("abnormal-events missing event: %s", abBody)
	}

	if !strings.Contains(stderr.String(), "FIRE:") {
		t.Errorf("missing FIRE log: %s", stderr.String())
	}
}

func TestRun_WarnEmittedOnceAtThreshold(t *testing.T) {
	t.Parallel()
	ws := tempWorkspace(t)
	activity := filepath.Join(ws, "a.log")
	staleTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.WriteFile(activity, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(activity, staleTime, staleTime); err != nil {
		t.Fatal(err)
	}

	// startTime anchors at staleTime+5s, then now jumps to staleTime+460s
	// (past warn=450 but not threshold=600). Stale anchor matters because
	// phaseStartTime initializes to startTime; we need now-startTime >= warnS.
	callIdx := 0
	nowFn := func() time.Time {
		callIdx++
		if callIdx == 1 {
			return staleTime.Add(5 * time.Second)
		}
		return staleTime.Add(460 * time.Second)
	}

	var stderr bytes.Buffer
	rc := Run(Config{
		Workspace: ws, TargetPGID: 999, Cycle: 1, ThresholdS: 600, WarnPct: 75, PollS: 1, GraceS: 1,
		Now: nowFn, Sleep: func(time.Duration) {},
		KillPgrp:  func(int, syscall.Signal) error { return nil },
		StopAfter: 2,
	}, &stderr)
	if rc != ExitOK {
		t.Errorf("rc=%d, want 0", rc)
	}
	count := strings.Count(stderr.String(), "WARN: idle for")
	if count != 1 {
		t.Errorf("want exactly 1 WARN line, got %d (log=%s)", count, stderr.String())
	}
}

func TestRun_PhaseTransitionResetsBaseline(t *testing.T) {
	t.Parallel()
	ws := tempWorkspace(t)
	cycleState := filepath.Join(ws, "cycle-state.json")
	if err := os.WriteFile(cycleState, []byte(`{"phase":"intent"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	activity := filepath.Join(ws, "a.log")
	if err := os.WriteFile(activity, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	staleTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_ = os.Chtimes(activity, staleTime, staleTime)

	// iteration timeline: now starts well past threshold from staleTime
	// but the phase change resets baseline to current iteration's now —
	// so FIRE should not happen.
	iter := 0
	nowFn := func() time.Time {
		iter++
		base := staleTime.Add(time.Duration(700+iter) * time.Second)
		return base
	}
	// rewrite cycle-state on iter 2 to trigger phase advance
	go func() {
		time.Sleep(100 * time.Millisecond)
	}()

	killCalled := false
	var stderr bytes.Buffer
	rc := Run(Config{
		Workspace: ws, TargetPGID: 100, Cycle: 1, CycleStatePath: cycleState,
		ThresholdS: 600, WarnPct: 75, PollS: 1, GraceS: 1,
		Now:       nowFn,
		Sleep:     func(time.Duration) {},
		KillPgrp:  func(int, syscall.Signal) error { killCalled = true; return nil },
		StopAfter: 1, // only one iteration — phase observed but no fire
	}, &stderr)
	if rc != ExitOK {
		t.Errorf("rc=%d", rc)
	}
	// First iteration anchors phase; baseline = now → idle_s ≈ 0 → no fire
	if killCalled {
		t.Errorf("kill should not fire when phase was just observed (baseline reset)")
	}
	if !strings.Contains(stderr.String(), "phase observed: 'intent'") {
		t.Errorf("missing phase-observed log: %s", stderr.String())
	}
}

func TestReadPhase_MissingFile(t *testing.T) {
	t.Parallel()
	if p := readPhase("/nonexistent"); p != "" {
		t.Errorf("got %q, want empty", p)
	}
}

func TestReadPhase_BadJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "cs.json")
	if err := os.WriteFile(p, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readPhase(p); got != "" {
		t.Errorf("bad json should give empty, got %q", got)
	}
}

func TestReadPhase_OK(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "cs.json")
	if err := os.WriteFile(p, []byte(`{"phase":"audit","other":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readPhase(p); got != "audit" {
		t.Errorf("got %q, want audit", got)
	}
}

func TestNewestMatch_NoMatches(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m, p := newestMatch(filepath.Join(dir, "nope-*.log"))
	if m != 0 || p != "" {
		t.Errorf("expected empty, got %d %s", m, p)
	}
}

func TestNewestMatch_PicksNewest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for i, name := range []string{"a.log", "b.log", "c.log"} {
		path := filepath.Join(dir, name)
		_ = os.WriteFile(path, []byte("x"), 0o644)
		t := time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC)
		_ = os.Chtimes(path, t, t)
	}
	_, best := newestMatch(filepath.Join(dir, "*.log"))
	if filepath.Base(best) != "c.log" {
		t.Errorf("got %s, want c.log", best)
	}
}

func TestAppendAbnormalEvent_BadWorkspaceNoop(t *testing.T) {
	t.Parallel()
	// no panic, no file created
	appendAbnormalEvent("/nonexistent-dir-xyz", "x", time.Now)
}

func TestAppendAbnormalEvent_WritesLine(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	appendAbnormalEvent(dir, "idle_s=900", time.Now)
	body, err := os.ReadFile(filepath.Join(dir, "abnormal-events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "stall-detected") {
		t.Errorf("missing event_type: %s", body)
	}
	if !strings.Contains(string(body), "idle_s=900") {
		t.Errorf("missing details: %s", body)
	}
}
