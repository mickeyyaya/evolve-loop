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

func TestMtimeUnix_EmptyPathIsZero(t *testing.T) {
	t.Parallel()
	if m := mtimeUnix(""); m != 0 {
		t.Errorf("empty path → 0, got %d", m)
	}
}

func TestMtimeUnix_StatErrorIsZero(t *testing.T) {
	t.Parallel()
	if m := mtimeUnix(filepath.Join(t.TempDir(), "absent.json")); m != 0 {
		t.Errorf("non-existent path → 0 (stat error branch), got %d", m)
	}
}

func TestMtimeUnix_ReturnsModTime(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "f.json")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := os.Chtimes(p, want, want); err != nil {
		t.Fatal(err)
	}
	if got := mtimeUnix(p); got != want.Unix() {
		t.Errorf("mtimeUnix=%d, want %d", got, want.Unix())
	}
}

// TestNewestMatch_GlobError — a malformed glob pattern (unterminated '[')
// returns the (0, "") zero result rather than panicking (phasewatchdog.go:265).
func TestNewestMatch_GlobError(t *testing.T) {
	t.Parallel()
	m, p := newestMatch(filepath.Join(t.TempDir(), "x[", "*.log"))
	if m != 0 || p != "" {
		t.Errorf("malformed glob → (0, \"\"), got (%d, %q)", m, p)
	}
}

// TestAppendAbnormalEvent_NilNowUsesWallClock — when now is nil the helper falls
// back to time.Now and still writes a timestamped event (phasewatchdog.go:286-288).
func TestAppendAbnormalEvent_NilNowUsesWallClock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	appendAbnormalEvent(dir, "idle_s=42", nil)
	body, err := os.ReadFile(filepath.Join(dir, "abnormal-events.jsonl"))
	if err != nil {
		t.Fatalf("event must be written even with nil now: %v", err)
	}
	if !strings.Contains(string(body), `"timestamp"`) {
		t.Errorf("nil-now event must still carry a timestamp; got %s", body)
	}
}

// TestAppendAbnormalEvent_OpenFileErrorNoop — when the target jsonl path already
// exists as a DIRECTORY, OpenFile fails and the helper returns silently without
// panicking and without leaving a stray file (phasewatchdog.go:303-305).
func TestAppendAbnormalEvent_OpenFileErrorNoop(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Block the append target by pre-creating it as a directory.
	if err := os.Mkdir(filepath.Join(dir, "abnormal-events.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Must not panic; the dir must remain a dir (no write).
	appendAbnormalEvent(dir, "idle_s=1", time.Now)
	info, err := os.Stat(filepath.Join(dir, "abnormal-events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Errorf("append target should remain a directory (OpenFile error path)")
	}
}

// TestDefaultKillPgrp_Signal0Succeeds — exercises the real defaultKillPgrp
// (phasewatchdog.go:312-314). Signal 0 is the POSIX existence probe: it performs
// no kill, so targeting this test process's own group is safe and deterministic,
// and it must return nil (the group exists).
func TestDefaultKillPgrp_Signal0Succeeds(t *testing.T) {
	t.Parallel()
	pgid, err := syscall.Getpgid(os.Getpid())
	if err != nil {
		t.Skipf("Getpgid unavailable: %v", err)
	}
	if err := defaultKillPgrp(pgid, syscall.Signal(0)); err != nil {
		t.Errorf("signal-0 existence probe of own pgid should succeed, got %v", err)
	}
}

// TestRun_PhaseAdvanceLogsTransition — when the phase in cycle-state.json changes
// from one non-empty value to another between iterations, Run logs the "phase
// advance: 'X' → 'Y'" line and resets the baseline (phasewatchdog.go:127-129).
// The Sleep seam rewrites the phase file between iterations to drive the change
// deterministically (no real sleep, no goroutine race).
func TestRun_PhaseAdvanceLogsTransition(t *testing.T) {
	// Not parallel: drives shared file state across iterations via the Sleep seam.
	ws := tempWorkspace(t)
	cycleState := filepath.Join(ws, "cycle-state.json")
	if err := os.WriteFile(cycleState, []byte(`{"phase":"build"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	nowFn := func() time.Time { return base.Add(1 * time.Second) }

	// After the first iteration's Sleep, flip the phase to "audit" so iteration 2
	// observes a non-empty→non-empty transition (the advance branch).
	sleeps := 0
	sleepFn := func(time.Duration) {
		sleeps++
		if sleeps == 1 {
			if err := os.WriteFile(cycleState, []byte(`{"phase":"audit"}`), 0o644); err != nil {
				t.Errorf("rewrite cycle-state: %v", err)
			}
		}
	}

	var stderr bytes.Buffer
	rc := Run(Config{
		Workspace: ws, TargetPGID: 7, Cycle: 1, CycleStatePath: cycleState,
		ThresholdS: 600, WarnPct: 75, PollS: 1, GraceS: 1,
		Now:       nowFn,
		Sleep:     sleepFn,
		KillPgrp:  func(int, syscall.Signal) error { return nil },
		StopAfter: 2,
	}, &stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d, want 0 (log=%s)", rc, stderr.String())
	}
	if !strings.Contains(stderr.String(), "phase observed: 'build'") {
		t.Errorf("first iteration should observe phase 'build'; log=%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "phase advance: 'build' → 'audit'") {
		t.Errorf("second iteration should log the advance to 'audit'; log=%s", stderr.String())
	}
}

// TestRun_CycleStateMtimeIsFreshestActivity — when cycle-state.json has the
// newest mtime (no log/md/json activity files beat it), it is selected as the
// last-activity baseline (phasewatchdog.go:148-151). With a fresh mtime the
// baseline is recent → no FIRE; the test asserts no kill and clean exit.
func TestRun_CycleStateMtimeIsFreshestActivity(t *testing.T) {
	t.Parallel()
	ws := tempWorkspace(t)
	// cycle-state.json lives OUTSIDE the workspace so the workspace "*.json" glob
	// does not catch it — that forces the explicit cycle-state mtime branch
	// (phasewatchdog.go:148-151) to be the source that beats bestMtime.
	cycleState := filepath.Join(t.TempDir(), "cycle-state.json")
	if err := os.WriteFile(cycleState, []byte(`{"phase":"build"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// Seed a stale workspace log so the (fresh) cycle-state is strictly newer.
	stale := filepath.Join(ws, "old.log")
	if err := os.WriteFile(stale, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	staleT := base.Add(-1000 * time.Second)
	if err := os.Chtimes(stale, staleT, staleT); err != nil {
		t.Fatal(err)
	}
	// cycle-state mtime equals "now" → idle ≈ 0 → never fires.
	if err := os.Chtimes(cycleState, base, base); err != nil {
		t.Fatal(err)
	}
	nowFn := func() time.Time { return base }

	killed := false
	var stderr bytes.Buffer
	rc := Run(Config{
		Workspace: ws, TargetPGID: 7, Cycle: 1, CycleStatePath: cycleState,
		ThresholdS: 600, WarnPct: 75, PollS: 1, GraceS: 1,
		Now:       nowFn,
		Sleep:     func(time.Duration) {},
		KillPgrp:  func(int, syscall.Signal) error { killed = true; return nil },
		StopAfter: 1,
	}, &stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d, want 0", rc)
	}
	if killed {
		t.Errorf("fresh cycle-state mtime should keep idle near 0 → no fire")
	}
}

// TestRun_LedgerMtimeIsFreshestActivity — when ProjectRoot is set and
// .evolve/ledger.jsonl has the newest mtime, the ledger is selected as the
// last-activity source (phasewatchdog.go:154-159). A fresh ledger mtime keeps
// idle near 0 → no fire.
func TestRun_LedgerMtimeIsFreshestActivity(t *testing.T) {
	t.Parallel()
	ws := tempWorkspace(t)
	projectRoot := t.TempDir()
	ledgerDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(ledgerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ledger := filepath.Join(ledgerDir, "ledger.jsonl")
	if err := os.WriteFile(ledger, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(ledger, base, base); err != nil {
		t.Fatal(err)
	}
	// Seed a stale workspace log so the ledger (fresh) is strictly newer.
	stale := filepath.Join(ws, "old.log")
	if err := os.WriteFile(stale, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	staleT := base.Add(-1000 * time.Second)
	if err := os.Chtimes(stale, staleT, staleT); err != nil {
		t.Fatal(err)
	}
	nowFn := func() time.Time { return base } // == ledger mtime → idle ≈ 0

	killed := false
	var stderr bytes.Buffer
	rc := Run(Config{
		Workspace: ws, TargetPGID: 7, Cycle: 1, ProjectRoot: projectRoot,
		ThresholdS: 600, WarnPct: 75, PollS: 1, GraceS: 1,
		Now:       nowFn,
		Sleep:     func(time.Duration) {},
		KillPgrp:  func(int, syscall.Signal) error { killed = true; return nil },
		StopAfter: 1,
	}, &stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d, want 0", rc)
	}
	if killed {
		t.Errorf("fresh ledger mtime should keep idle near 0 → no fire")
	}
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
