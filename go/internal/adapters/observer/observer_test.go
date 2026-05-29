// Package observer ports the per-phase observer service from
// scripts/dispatch/phase-observer.sh. Phase 2 ships the minimal
// goroutine skeleton (stall detection + NDJSON events); the full
// 5-rule detector suite (infinite_loop, error_spike, cost_anomaly,
// rate_limit) is Phase 3 follow-up.
package observer

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestNew_Defaults — empty Config uses bash-defaults from CLAUDE.md
// env-var table: StallS=600s, PollS=5s.
func TestNew_Defaults(t *testing.T) {
	o := New(Config{}, &bytes.Buffer{})
	if o == nil {
		t.Fatal("New=nil")
	}
	if o.cfg.StallS != DefaultStallS {
		t.Errorf("StallS=%v, want %v", o.cfg.StallS, DefaultStallS)
	}
	if o.cfg.PollS != DefaultPollS {
		t.Errorf("PollS=%v, want %v", o.cfg.PollS, DefaultPollS)
	}
}

// TestWatch_StallEmitsIncident drives a deterministic stall: with a
// virtual clock advanced past StallS, Watch emits an "incident" event
// reporting stall_no_output. Mirrors bash phase-observer.sh:stall rule.
func TestWatch_StallEmitsIncident(t *testing.T) {
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "scout-stdout.log")
	// Pre-create the log so the observer has something to stat. No growth
	// after this point.
	if err := os.WriteFile(logFile, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}
	var sink bytes.Buffer
	o := New(Config{
		StallS:    100 * time.Millisecond,
		PollS:     10 * time.Millisecond,
		Cycle:     7,
		Phase:     "scout",
		Agent:     "evolve-scout",
		StdoutLog: logFile,
	}, &sink)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := o.Watch(ctx); err != nil && err != context.DeadlineExceeded {
		t.Fatalf("Watch: %v", err)
	}
	// Expect at least one stall event in the sink.
	events := parseEvents(t, sink.Bytes())
	hasStall := false
	for _, e := range events {
		if e.Type == "stall_no_output" && e.Severity == "incident" {
			hasStall = true
			break
		}
	}
	if !hasStall {
		t.Errorf("no stall event in %d events: %+v", len(events), events)
	}
}

// TestWatch_GrowthResetsStallTimer — when stdout log grows, the stall
// timer resets. Test by appending bytes mid-watch and asserting no
// stall fires in the short window.
func TestWatch_GrowthResetsStallTimer(t *testing.T) {
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "scout-stdout.log")
	if err := os.WriteFile(logFile, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	var sink syncBuffer
	o := New(Config{
		StallS: 200 * time.Millisecond,
		PollS:  20 * time.Millisecond,
		Cycle:  1, Phase: "scout", Agent: "x",
		StdoutLog: logFile,
	}, &sink)

	// Goroutine: append every 50ms for 250ms — total work fits inside
	// 300ms watch window with reset preventing stall.
	stop := make(chan struct{})
	go func() {
		tick := time.NewTicker(50 * time.Millisecond)
		defer tick.Stop()
		for i := 0; i < 5; i++ {
			select {
			case <-stop:
				return
			case <-tick.C:
				f, _ := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0o644)
				_, _ = f.Write([]byte("more "))
				_ = f.Close()
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_ = o.Watch(ctx)
	close(stop)
	for _, e := range parseEvents(t, sink.bytes()) {
		if e.Type == "stall_no_output" {
			t.Errorf("stall fired during continuous growth: %+v", e)
		}
	}
}

// TestWatch_WorkspaceActivityResetsStallTimer — cycle-141: a tmux-driver
// agent writes its live output to the tmux scrollback, NOT the stdout-log,
// so the stdout-log stays flat while the agent is productively writing
// artifacts (worktree commit, reflection.yaml) into the workspace tree. When
// WorkspaceDir is set, a fresh write anywhere under it counts as progress and
// resets the stall timer — so a working tmux agent is not falsely killed.
func TestWatch_WorkspaceActivityResetsStallTimer(t *testing.T) {
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "build-stdout.log")
	if err := os.WriteFile(logFile, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}
	var sink syncBuffer
	o := New(Config{
		StallS: 200 * time.Millisecond,
		PollS:  20 * time.Millisecond,
		Cycle:  1, Phase: "build", Agent: "build",
		StdoutLog:    logFile,
		WorkspaceDir: tmp,
	}, &sink)

	// Append to a WORKSPACE artifact (not the stdout-log) every 50ms. The
	// stdout-log never grows, but the workspace mtime advances → no stall.
	stop := make(chan struct{})
	go func() {
		tick := time.NewTicker(50 * time.Millisecond)
		defer tick.Stop()
		for i := 0; i < 5; i++ {
			select {
			case <-stop:
				return
			case <-tick.C:
				p := filepath.Join(tmp, "build-reflection.yaml")
				f, _ := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
				_, _ = f.Write([]byte("line\n"))
				_ = f.Close()
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_ = o.Watch(ctx)
	close(stop)
	for _, e := range parseEvents(t, sink.bytes()) {
		if e.Type == "stall_no_output" {
			t.Errorf("stall fired while the agent was writing workspace artifacts: %+v", e)
		}
	}
}

// TestWatch_WorkspaceConfiguredButIdle_StillStalls — guard against the
// activity signal disabling stall detection: with WorkspaceDir set but no
// writes anywhere, a genuine stall must still fire.
func TestWatch_WorkspaceConfiguredButIdle_StillStalls(t *testing.T) {
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "build-stdout.log")
	if err := os.WriteFile(logFile, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}
	var sink bytes.Buffer
	o := New(Config{
		StallS: 100 * time.Millisecond,
		PollS:  10 * time.Millisecond,
		Cycle:  9, Phase: "build", Agent: "build",
		StdoutLog:    logFile,
		WorkspaceDir: tmp,
	}, &sink)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = o.Watch(ctx)
	hasStall := false
	for _, e := range parseEvents(t, sink.Bytes()) {
		if e.Type == "stall_no_output" && e.Severity == "incident" {
			hasStall = true
		}
	}
	if !hasStall {
		t.Error("idle agent with WorkspaceDir set must still stall")
	}
}

// TestWatch_ObserverEventsFileDoesNotMaskStall — the observer's own events
// sink, when it lives inside WorkspaceDir, must be excluded from the activity
// scan. Otherwise the "started" (and later "stall") writes would advance the
// workspace mtime and the observer could never detect a stall — it would keep
// resetting on its own writes. Simulate by pre-creating the events file and
// touching it on the same cadence the observer would; a real stall must fire.
func TestWatch_ObserverEventsFileDoesNotMaskStall(t *testing.T) {
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "build-stdout.log")
	if err := os.WriteFile(logFile, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The events file the production adapter creates: <phase>-observer-events.ndjson.
	eventsFile := filepath.Join(tmp, "build-observer-events.ndjson")
	var sink syncBuffer
	o := New(Config{
		StallS: 150 * time.Millisecond,
		PollS:  10 * time.Millisecond,
		Cycle:  3, Phase: "build", Agent: "build",
		StdoutLog:    logFile,
		WorkspaceDir: tmp,
	}, &sink)

	stop := make(chan struct{})
	go func() {
		tick := time.NewTicker(20 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				f, _ := os.OpenFile(eventsFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
				_, _ = f.Write([]byte("{}\n"))
				_ = f.Close()
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	_ = o.Watch(ctx)
	close(stop)
	hasStall := false
	for _, e := range parseEvents(t, sink.bytes()) {
		if e.Type == "stall_no_output" {
			hasStall = true
		}
	}
	if !hasStall {
		t.Error("stall must fire even though the observer's own events file keeps growing (it must be excluded from the activity scan)")
	}
}

// TestStop_StopsBeforeStallFires — calling Stop() interrupts the
// observer before any stall can be detected.
func TestStop_StopsBeforeStallFires(t *testing.T) {
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "x.log")
	_ = os.WriteFile(logFile, []byte("x"), 0o644)
	var sink bytes.Buffer
	o := New(Config{
		StallS: time.Hour, PollS: 10 * time.Millisecond,
		Cycle: 1, Phase: "x", Agent: "x", StdoutLog: logFile,
	}, &sink)

	ctx := context.Background()
	done := make(chan error, 1)
	go func() { done <- o.Watch(ctx) }()

	time.Sleep(30 * time.Millisecond)
	if err := o.Stop(); err != nil {
		t.Errorf("Stop: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Watch returned: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Watch did not return after Stop()")
	}
}

// TestWatch_ContextCancelReturnsErr — context cancellation propagates.
func TestWatch_ContextCancelReturnsErr(t *testing.T) {
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "x.log")
	_ = os.WriteFile(logFile, []byte("x"), 0o644)
	o := New(Config{
		StallS: time.Hour, PollS: 10 * time.Millisecond,
		StdoutLog: logFile,
	}, &bytes.Buffer{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := o.Watch(ctx)
	if err == nil {
		t.Error("Watch with canceled ctx: want error")
	}
}

// TestWatch_MissingLogFile — when StdoutLog doesn't exist, Watch
// either tolerates it (file may be created mid-watch) or surfaces an
// initial-stat error. The contract: never panic.
func TestWatch_MissingLogFile(t *testing.T) {
	o := New(Config{
		StallS: 50 * time.Millisecond, PollS: 10 * time.Millisecond,
		Cycle: 1, Phase: "x", Agent: "x",
		StdoutLog: "/no/such/log",
	}, &bytes.Buffer{})
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	// Should not panic, should return cleanly on ctx timeout.
	_ = o.Watch(ctx)
}

// TestEvent_NDJSONFormat — each emitted event is one valid JSON object
// per line. The aggregator (bash) parses NDJSON; the Go port must
// preserve that line discipline.
func TestEvent_NDJSONFormat(t *testing.T) {
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "x.log")
	_ = os.WriteFile(logFile, []byte("x"), 0o644)
	var sink bytes.Buffer
	o := New(Config{
		StallS: 50 * time.Millisecond, PollS: 10 * time.Millisecond,
		Cycle: 1, Phase: "scout", Agent: "evolve-scout",
		StdoutLog: logFile,
	}, &sink)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	_ = o.Watch(ctx)
	for _, line := range strings.Split(strings.TrimSpace(sink.String()), "\n") {
		if line == "" {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Errorf("NDJSON line not valid JSON: %q: %v", line, err)
		}
	}
}

// Helpers ------------------------------------------------------------

func parseEvents(t *testing.T, b []byte) []Event {
	t.Helper()
	var out []Event
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Logf("skip malformed line: %q", line)
			continue
		}
		out = append(out, e)
	}
	return out
}

// syncBuffer wraps bytes.Buffer with a mutex so concurrent writes
// (observer goroutine + test goroutine appending to log) don't race.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := make([]byte, s.buf.Len())
	copy(b, s.buf.Bytes())
	return b
}
