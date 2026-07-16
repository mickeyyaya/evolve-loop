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
// emit, the EOF-grace shutdown, the tail rotation/stat-miss paths, and the
// processLine empty-content / empty-tool-name edges. Behavior-pinned, no real
// sleeps > ~150ms, deterministic clocks and t.TempDir only.

// fixedClock returns a Now func pinned to a single instant — used where the
// test asserts on emitted timestamps or needs the idle clock to stay frozen.
func fixedClock(at time.Time) func() time.Time {
	return func() time.Time { return at }
}

// === Run applies zero-value defaults ========================================
// Passing a Config with all the tunable knobs left at their zero values forces
// every `if cfg.X == 0 { cfg.X = default }` branch in Run (PollS, StallS,
// LoopN, LoopWindowS, ErrorRate, CostSigma, ThrottleN, EOFGraceS,
// HeartbeatEvery, Scope) plus the empty-stdoutPath default. A quick SIGUSR1
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

// === tail: stat-miss returns the prior offset ===============================
// A missing stdout path makes os.Stat fail, so tail returns no lines and the
// unchanged offset (332-335).
func TestTail_StatMissReturnsPriorOffset(t *testing.T) {
	t.Parallel()
	o := &Observer{lastByteOff: 42}
	lines, off := o.tail(filepath.Join(t.TempDir(), "does-not-exist.log"))
	if lines != nil {
		t.Errorf("lines = %v, want nil", lines)
	}
	if off != 42 {
		t.Errorf("offset = %d, want 42 (prior offset preserved on stat miss)", off)
	}
}

// === tail: rotation resets the offset to 0 and re-reads ====================
// When the file shrinks below the saved offset (log rotation), tail must reset
// lastByteOff to 0 and re-read from the top (336-339).
func TestTail_RotationResetsOffset(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "stdout.log")
	if err := os.WriteFile(p, []byte("line-after-rotation\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Saved offset is past the new (smaller) file size → rotation detected.
	o := &Observer{lastByteOff: 9999}
	lines, off := o.tail(p)
	if len(lines) != 1 || lines[0] != "line-after-rotation" {
		t.Fatalf("lines = %v, want [line-after-rotation] (rotation re-read from 0)", lines)
	}
	if off != int64(len("line-after-rotation\n")) {
		t.Errorf("offset = %d, want %d", off, len("line-after-rotation\n"))
	}
	if o.lastByteOff != 0 {
		t.Errorf("lastByteOff = %d, want 0 (reset on rotation)", o.lastByteOff)
	}
}

// === processLine: assistant with empty content array ========================
// An assistant message whose content slice is empty must early-return after
// the eventCount bump, leaving no tool-call recorded (376-378).
func TestProcessLine_AssistantEmptyContent(t *testing.T) {
	t.Parallel()
	o := &Observer{cfg: Config{Now: time.Now}}
	o.processLine(`{"type":"assistant","message":{"content":[]}}`)
	if o.eventCount != 1 {
		t.Errorf("eventCount = %d, want 1 (line counted)", o.eventCount)
	}
	if o.toolCallCount != 0 || len(o.loopHistory) != 0 {
		t.Errorf("empty content must not record a tool call; tc=%d hist=%d",
			o.toolCallCount, len(o.loopHistory))
	}
}

// === processLine: tool_use with no name defaults to "?" =====================
// A tool_use block missing its name still counts as a tool call and records a
// loop-history entry tagged "?" (383-385).
func TestProcessLine_ToolUseMissingNameDefaultsQuestion(t *testing.T) {
	t.Parallel()
	o := &Observer{cfg: Config{Now: time.Now}}
	o.processLine(`{"type":"assistant","message":{"content":[{"type":"tool_use","input":{}}]}}`)
	if o.toolCallCount != 1 {
		t.Fatalf("toolCallCount = %d, want 1", o.toolCallCount)
	}
	if len(o.loopHistory) != 1 || o.loopHistory[0].tool != "?" {
		t.Errorf("loopHistory = %+v, want one entry tool=%q", o.loopHistory, "?")
	}
}

// === processLine: user message with empty content array =====================
// A user message with empty content early-returns after the eventCount bump,
// recording no tool result (398-400).
func TestProcessLine_UserEmptyContent(t *testing.T) {
	t.Parallel()
	o := &Observer{cfg: Config{Now: time.Now}}
	o.processLine(`{"type":"user","message":{"content":[]}}`)
	if o.eventCount != 1 {
		t.Errorf("eventCount = %d, want 1", o.eventCount)
	}
	if o.toolResultCnt != 0 || o.errorCount != 0 {
		t.Errorf("empty user content must not record a tool result; tr=%d err=%d",
			o.toolResultCnt, o.errorCount)
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
// so writeReport returns an error and Run takes the WARN branch (324-326)
// while still exiting ExitOK (report write is best-effort).
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
// OpenFile fail, so the nudge-append error branch (264-266) logs a WARN. The
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
