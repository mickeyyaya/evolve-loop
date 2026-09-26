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

func fixedClock(at time.Time) func() time.Time {
	return func() time.Time { return at }
}

func TestRun_AppliesZeroValueDefaults(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	shutdown := make(chan struct{})
	at := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)

	var rc int
	done := make(chan struct{})
	go func() {
		rc = Run(Config{
			Workspace: ws, Cycle: 1, Phase: "build", Agent: "builder",
			Now:         fixedClock(at),
			ShutdownSig: shutdown,
		}, "", os.Stderr)
		close(done)
	}()
	// An already-closed shutdown fires on the first select, so <-done is the only barrier needed.
	close(shutdown)
	<-done

	if rc != ExitOK {
		t.Fatalf("rc=%d, want ExitOK", rc)
	}
	if _, err := os.ReadFile(filepath.Join(ws, "builder-observer-report.json")); err != nil {
		t.Fatalf("report missing (defaults path broke?): %v", err)
	}
}

func TestRun_HeartbeatEmits(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	at := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	rc := Run(Config{
		Workspace: ws, Cycle: 1, Phase: "build", Agent: "builder",
		PollS: 1, StallS: 9999, EOFGraceS: 9999,
		HeartbeatEvery: 1,
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

func TestRun_EOFGraceShutdown(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	stdoutPath := filepath.Join(ws, "builder-stdout.log")
	// EOF grace needs at least one event, so seed one.
	if err := os.WriteFile(stdoutPath, []byte(
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{}}]}}`+"\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	at := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	rc := Run(Config{
		Workspace: ws, Cycle: 1, Phase: "build", Agent: "builder",
		PollS: 1, StallS: 9999,
		EOFGraceS:   1,
		Now:         fixedClock(at),
		StopAfterMS: 4000, // long enough that EOF grace wins
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

func TestRun_InstallsDefaultKillPgrp(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	at := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	rc := Run(Config{
		Workspace: ws, Cycle: 1, Phase: "build", Agent: "builder",
		PollS: 1, StallS: 9999, EOFGraceS: 9999,
		// The real kill default is installed but never sent: the clock is frozen and the pgid is 0.
		Now:         fixedClock(at),
		StopAfterMS: 120,
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
}

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
			ShutdownSig: shutdown,
		}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
		close(done)
	}()
	// An already-closed shutdown fires on the first select, so <-done is the only barrier needed.
	close(shutdown)
	<-done
	if rc != ExitOK {
		t.Fatalf("rc=%d, want ExitOK with defaulted clock", rc)
	}
}

func TestRun_WriteReportFailure_WarnsButReturnsOK(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	// A directory at the report path makes the final os.Rename fail.
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

func TestRun_SoftStallNudge_AppendFailureWarns(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	// A directory at inbox.Path(ws, "builder") makes inbox.Append's open fail.
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
	events, _ := os.ReadFile(filepath.Join(ws, "builder-observer-events.ndjson"))
	if !strings.Contains(string(events), "soft_stall_nudge") {
		t.Errorf("expected soft_stall_nudge event despite append failure:\n%s", events)
	}
}
