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

// coverage_test.go targets branches the behavior suite does not yet reach:
// the zero-value config defaults, the empty-stdoutPath default, the heartbeat
// emit, the EOF-grace shutdown and the two fault paths the engine reports
// (the tail/processLine edges moved to internal/observerengine, ADR-0103
// unit 12). Behavior-pinned, no real
// sleeps > ~150ms, deterministic clocks and t.TempDir only.

// fixedClock returns a Now func pinned to a single instant — used where the
// test asserts on emitted timestamps or needs the idle clock to stay frozen.
func fixedClock(at time.Time) func() time.Time {
	return func() time.Time { return at }
}

// === Run applies zero-value defaults ========================================
// Passing a Config with all the tunable knobs left at their zero values forces
// every `if cfg.X == 0 { cfg.X = default }` branch in withDefaults (PollS,
// StallS, EOFGraceS, HeartbeatEvery, Scope — the five never-read defaults
// LoopN/LoopWindowS/ErrorRate/CostSigma/ThrottleN fell with ADR-0103 unit
// 12) plus the empty-stdoutPath default. A quick SIGUSR1
// shutdown keeps it deterministic.
func TestRun_AppliesZeroValueDefaults(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	shutdown := make(chan struct{})
	at := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)

	var rc int
	done := make(chan struct{})
	go func() {
		// stdoutPath "" → Run defaults it to <ws>/<agent>-stdout.log (196-198).
		rc = Run(Config{
			Workspace: ws, Cycle: 1, Phase: "build", Agent: "builder",
			// All tunables zero → defaults applied.
			Now:         fixedClock(at),
			ShutdownSig: shutdown,
		}, "", os.Stderr)
		close(done)
	}()
	// close(shutdown) is caught on the poll loop's first select iteration (an
	// already-closed channel fires immediately); <-done is the real barrier —
	// no sleep needed.
	close(shutdown)
	<-done

	if rc != ExitOK {
		t.Fatalf("rc=%d, want ExitOK", rc)
	}
	// The report is still written under the defaulted agent filename.
	if _, err := os.ReadFile(filepath.Join(ws, "builder-observer-report.json")); err != nil {
		t.Fatalf("report missing (defaults path broke?): %v", err)
	}
}

// === Heartbeat emits on the HeartbeatEvery boundary =========================
// HeartbeatEvery=1 makes every poll tick a heartbeat boundary, so the
// heartbeat emit (306-313) fires. The frozen clock keeps StallS from tripping.
func TestRun_HeartbeatEmits(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	at := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	rc := Run(Config{
		Workspace: ws, Cycle: 1, Phase: "build", Agent: "builder",
		PollS: 1, StallS: 9999, EOFGraceS: 9999,
		HeartbeatEvery: 1, // every poll tick is a heartbeat
		Now:            fixedClock(at),
		StopAfterMS:    400,
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	events, err := os.ReadFile(filepath.Join(ws, "builder-observer-events.ndjson"))
	if err != nil {
		t.Fatalf("events missing: %v", err)
	}
	if !strings.Contains(string(events), `"heartbeat"`) {
		t.Errorf("expected a heartbeat event:\n%s", events)
	}
}

// === EOF-grace shutdown after stdout stops growing ==========================
// With events already seen (eventCount>0) and the stdout log not growing, the
// quiet-tick counter reaches EOFGraceS and Run shuts down with reason
// "eof_grace" (316-318) rather than waiting for the stop timer.
func TestRun_EOFGraceShutdown(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	stdoutPath := filepath.Join(ws, "builder-stdout.log")
	// One real event so eventCount > 0 (EOF-grace is gated on having seen output).
	if err := os.WriteFile(stdoutPath, []byte(
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{}}]}}`+"\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	at := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	rc := Run(Config{
		Workspace: ws, Cycle: 1, Phase: "build", Agent: "builder",
		PollS: 1, StallS: 9999,
		EOFGraceS:   1, // a single quiet tick * PollS(1) >= 1 → EOF grace fires
		Now:         fixedClock(at),
		StopAfterMS: 4000, // generous: EOF-grace should win first
	}, stdoutPath, os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	events, err := os.ReadFile(filepath.Join(ws, "builder-observer-events.ndjson"))
	if err != nil {
		t.Fatalf("events missing: %v", err)
	}
	if !strings.Contains(string(events), "eof_grace") {
		t.Errorf("expected observer_shutdown reason eof_grace:\n%s", events)
	}
}

// === Stop-timer shutdown path ===============================================
// With no ShutdownSig and a frozen clock (no stall), the StopAfterMS timer is
// the only way out — exercising the stop-timer case (230-238).
func TestRun_StopTimerShutdown(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	at := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	rc := Run(Config{
		Workspace: ws, Cycle: 1, Phase: "build", Agent: "builder",
		PollS: 1, StallS: 9999, EOFGraceS: 9999,
		Now:         fixedClock(at),
		StopAfterMS: 120,
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	events, err := os.ReadFile(filepath.Join(ws, "builder-observer-events.ndjson"))
	if err != nil {
		t.Fatalf("events missing: %v", err)
	}
	if !strings.Contains(string(events), "stop-timer") {
		t.Errorf("expected observer_shutdown reason stop-timer:\n%s", events)
	}
}

// === KillPgrp default seam is installed when nil ============================
// Leaving KillPgrp nil forces Run to install its syscall-backed default
// (179-181). We never let the stall fire (frozen clock), so the default closure
// is wired but never invoked — proving the nil-default branch runs without a
// real signal being sent. SubagentPGID 0 also keeps the kill guard closed.
func TestRun_InstallsDefaultKillPgrp(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	at := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	rc := Run(Config{
		Workspace: ws, Cycle: 1, Phase: "build", Agent: "builder",
		PollS: 1, StallS: 9999, EOFGraceS: 9999,
		// KillPgrp nil → default installed; never fires (frozen clock, pgid 0).
		Now:         fixedClock(at),
		StopAfterMS: 120,
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
}

// === Run installs the real-clock default when Now is nil ====================
// Omitting Now forces the `cfg.Now == nil` branch (175-177) to install
// time.Now. A short real shutdown keeps the test bounded; the only assertion
// is that Run completes cleanly with the defaulted clock.
func TestRun_InstallsDefaultNowClock(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	shutdown := make(chan struct{})
	var rc int
	done := make(chan struct{})
	go func() {
		rc = Run(Config{
			Workspace: ws, Cycle: 1, Phase: "build", Agent: "builder",
			PollS: 1, StallS: 9999, EOFGraceS: 9999,
			// Now nil → Run installs time.Now (175-177).
			ShutdownSig: shutdown,
		}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
		close(done)
	}()
	// <-done is the barrier; an already-closed shutdown fires on the first
	// select iteration, so no startup sleep is needed.
	close(shutdown)
	<-done
	if rc != ExitOK {
		t.Fatalf("rc=%d, want ExitOK with defaulted clock", rc)
	}
}

// === writeReport failure is logged as WARN, Run still returns ExitOK ========
// Pre-creating the report path as a DIRECTORY makes the final os.Rename fail,
// so the engine reports OBSERVER_REPORT_WRITE_FAILED and Run still exits
// ExitOK (report write is best-effort).
func TestRun_WriteReportFailure_WarnsButReturnsOK(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	// Block the rename target: a directory cannot be replaced by os.Rename(file).
	if err := os.Mkdir(filepath.Join(ws, "builder-observer-report.json"), 0o755); err != nil {
		t.Fatalf("seed report-dir: %v", err)
	}
	at := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	rc := Run(Config{
		Workspace: ws, Cycle: 1, Phase: "build", Agent: "builder",
		PollS: 1, StallS: 9999, EOFGraceS: 9999,
		Now:         fixedClock(at),
		StopAfterMS: 120,
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Errorf("rc=%d, want ExitOK (report write is best-effort)", rc)
	}
}

// === soft-stall nudge append failure is logged, kill still gated ===========
// Pre-creating the inbox file path as a DIRECTORY makes inbox.Append's
// OpenFile fail, so the engine reports OBSERVER_NUDGE_APPEND_FAILED. The
// nudged flag is still set, and the clock stays below StallS so no kill fires.
func TestRun_SoftStallNudge_AppendFailureWarns(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	// inbox.Path(ws,"builder") == <ws>/.bridge-inbox/builder.ndjson — create it
	// as a directory so the inbox write cannot open it as a file.
	inboxFile := filepath.Join(ws, ".bridge-inbox", "builder.ndjson")
	if err := os.MkdirAll(inboxFile, 0o755); err != nil {
		t.Fatalf("seed inbox-dir: %v", err)
	}
	mu := &sync.Mutex{}
	startTime := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	callIdx := 0
	nowFn := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		callIdx++
		if callIdx <= 2 {
			return startTime
		}
		return startTime.Add(400 * time.Second) // > NudgeS(300), < StallS(600)
	}
	killCalls := 0
	rc := Run(Config{
		Workspace: ws, SubagentPGID: 99999, Cycle: 1,
		Phase: "build", Agent: "builder",
		PollS: 1, StallS: 600, NudgeS: 300, EOFGraceS: 9999,
		Enforce: true,
		Now:     nowFn,
		KillPgrp: func(int, syscall.Signal) error {
			mu.Lock()
			killCalls++
			mu.Unlock()
			return nil
		},
		StopAfterMS: 400,
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	mu.Lock()
	defer mu.Unlock()
	if killCalls != 0 {
		t.Errorf("kill must NOT fire below StallS; got %d", killCalls)
	}
	// The soft_stall_nudge event still emits even though the inbox append failed.
	events, _ := os.ReadFile(filepath.Join(ws, "builder-observer-events.ndjson"))
	if !strings.Contains(string(events), "soft_stall_nudge") {
		t.Errorf("expected soft_stall_nudge event despite append failure:\n%s", events)
	}
}
