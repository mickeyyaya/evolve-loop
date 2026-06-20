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
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestNew_Defaults — empty Config uses bash-defaults from CLAUDE.md
// env-var table: StallS=600s, PollS=5s.
func TestNew_Defaults(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// TestWatch_LivenessProbeSuppressesFalseStall — cycle-190 regression: a
// tmux-driver agent in a long single "Incubating" turn (extended thinking +
// one big tool call) commits NO scrollback lines and writes NO workspace
// artifact for minutes, then dumps everything at the end. Both filesystem
// liveness signals (stdout-log size, workspace mtime) stay flat, so the
// observer falsely fires stall_no_output. When a LivenessProbe reports the
// agent is still alive (e.g. the tmux pane spinner/token-counter advancing),
// the observer must HOLD the stall: reset the clock and emit a benign
// stall_probe_active info event instead of a false incident.
func TestWatch_LivenessProbeSuppressesFalseStall(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "tdd-stdout.log")
	// Flat log, no growth, no WorkspaceDir activity — exactly the cycle-190
	// think-then-dump window.
	if err := os.WriteFile(logFile, []byte("start"), 0o644); err != nil {
		t.Fatal(err)
	}
	var sink syncBuffer
	o := New(Config{
		StallS:    60 * time.Millisecond,
		PollS:     10 * time.Millisecond,
		Cycle:     190,
		Phase:     "tdd",
		Agent:     "tdd",
		StdoutLog: logFile,
		// Agent is alive mid-turn (tmux pane changing) though the filesystem
		// shows nothing yet.
		LivenessProbe: func() bool { return true },
	}, &sink)

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	_ = o.Watch(ctx)

	var sawStall, sawProbeActive bool
	for _, e := range parseEvents(t, sink.bytes()) {
		switch e.Type {
		case "stall_no_output":
			sawStall = true
		case "stall_probe_active":
			sawProbeActive = true
		}
	}
	if sawStall {
		t.Error("stall_no_output fired despite liveness probe reporting active (cycle-190 false-positive stall)")
	}
	if !sawProbeActive {
		t.Error("expected a stall_probe_active liveness event when the probe holds the kill")
	}
}

// TestWatch_LivenessProbeFalseStillStalls — the probe only ever SUPPRESSES a
// stall on positive liveness. A probe that reports inactive (no tmux session,
// pane unchanged) must NOT mask a genuine stall: stall_no_output still fires.
func TestWatch_LivenessProbeFalseStillStalls(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "tdd-stdout.log")
	if err := os.WriteFile(logFile, []byte("start"), 0o644); err != nil {
		t.Fatal(err)
	}
	var sink syncBuffer
	o := New(Config{
		StallS:        60 * time.Millisecond,
		PollS:         10 * time.Millisecond,
		Cycle:         1,
		Phase:         "tdd",
		Agent:         "tdd",
		StdoutLog:     logFile,
		LivenessProbe: func() bool { return false },
	}, &sink)

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	_ = o.Watch(ctx)

	saw := false
	for _, e := range parseEvents(t, sink.bytes()) {
		if e.Type == "stall_no_output" && e.Severity == "incident" {
			saw = true
		}
	}
	if !saw {
		t.Error("stall_no_output must still fire when the liveness probe reports inactive")
	}
}

// TestWatch_WorkspaceActivityResetsStallTimer — cycle-141: a tmux-driver
// agent writes its live output to the tmux scrollback, NOT the stdout-log,
// so the stdout-log stays flat while the agent is productively writing
// artifacts (worktree commit, reflection.yaml) into the workspace tree. When
// WorkspaceDir is set, a fresh write anywhere under it counts as progress and
// resets the stall timer — so a working tmux agent is not falsely killed.
func TestWatch_WorkspaceActivityResetsStallTimer(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

// awaitEvent blocks until an event of eventType is delivered on ch, or timeout
// elapses. The timeout is a GENEROUS safety bound, not a tuned window: the check
// passes the instant the trigger fires (however slow the -race / CI runner) and
// only fails if the event never fires. This replaces fixed wall-clock windows —
// the source of the macOS -race flakiness — with event-triggered synchronization.
func awaitEvent(t *testing.T, ch <-chan Event, eventType string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		select {
		case e := <-ch:
			if e.Type == eventType {
				return true
			}
		case <-deadline.C:
			return false
		}
	}
}

// TestWatch_ObserverEventsFileDoesNotMaskStall — the observer's own events
// sink, when it lives inside WorkspaceDir, must be excluded from the activity
// scan. Otherwise the "started" (and later "stall") writes would advance the
// workspace mtime and the observer could never detect a stall — it would keep
// resetting on its own writes. Simulate by pre-creating the events file and
// touching it on the same cadence the observer would; a real stall must fire.
func TestWatch_ObserverEventsFileDoesNotMaskStall(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "build-stdout.log")
	if err := os.WriteFile(logFile, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The observer's OWN events file: it must be EXCLUDED from the activity scan,
	// so its growth can never reset the stall timer and mask a genuine stall.
	eventsFile := filepath.Join(tmp, "build-observer-events.ndjson")

	events := make(chan Event, 64)
	var sink syncBuffer
	o := New(Config{
		StallS: 40 * time.Millisecond, PollS: 5 * time.Millisecond,
		Cycle: 3, Phase: "build", Agent: "build",
		StdoutLog: logFile, WorkspaceDir: tmp,
		// Non-blocking subscriber — the event-triggered result check reads this.
		OnEvent: func(e Event) {
			select {
			case events <- e:
			default:
			}
		},
	}, &sink)

	// Keep the observer's own events file growing throughout — the masking source.
	stopWriter := make(chan struct{})
	go func() {
		tick := time.NewTicker(5 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stopWriter:
				return
			case <-tick.C:
				f, err := os.OpenFile(eventsFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
				if err != nil {
					continue
				}
				_, _ = f.Write([]byte("{}\n"))
				_ = f.Close()
			}
		}
	}()
	defer close(stopWriter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = o.Watch(ctx) }()
	defer func() { _ = o.Stop() }()

	// Event-triggered: pass the instant the stall fires; the generous bound only
	// trips on a real regression (events-file growth wrongly masking the stall).
	if !awaitEvent(t, events, "stall_no_output", 5*time.Second) {
		t.Error("stall must fire even though the observer's own events file keeps growing (it must be excluded from the activity scan)")
	}
}

// TestStop_StopsBeforeStallFires — calling Stop() interrupts the
// observer before any stall can be detected.
func TestStop_StopsBeforeStallFires(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// TestStatSize_EmptyStdoutLogReturnsZero pins the early-return guard in
// statSize: an Observer with no StdoutLog configured reports size 0 (the
// "no output" sentinel) rather than calling os.Stat("").
func TestStatSize_EmptyStdoutLogReturnsZero(t *testing.T) {
	t.Parallel()
	o := New(Config{StdoutLog: ""}, &bytes.Buffer{})
	if got := o.statSize(); got != 0 {
		t.Errorf("statSize with empty StdoutLog = %d, want 0", got)
	}
}

// TestStatSize_MissingFileReturnsZero pins the os.Stat-error branch: a
// configured-but-absent log file also reports 0 (treated as no output; the
// ticker retries on a later poll once the runner creates the file).
func TestStatSize_MissingFileReturnsZero(t *testing.T) {
	t.Parallel()
	o := New(Config{StdoutLog: filepath.Join(t.TempDir(), "never-created.log")}, &bytes.Buffer{})
	if got := o.statSize(); got != 0 {
		t.Errorf("statSize on missing file = %d, want 0", got)
	}
}

// TestNewestActivity_UnsetWorkspaceReturnsZeroTime pins that the activity
// signal is disabled (zero Time) when WorkspaceDir is unset — stdout-log
// growth then governs stall detection alone.
func TestNewestActivity_UnsetWorkspaceReturnsZeroTime(t *testing.T) {
	t.Parallel()
	o := New(Config{WorkspaceDir: ""}, &bytes.Buffer{})
	if got := o.newestActivity(); !got.IsZero() {
		t.Errorf("newestActivity with unset WorkspaceDir = %v, want zero time", got)
	}
}

// TestNewestActivity_ReportsNewestFileMtime pins the happy path: the newest
// regular file's mtime is returned, and the observer's own events file is
// excluded from the scan (so its writes can never reset the stall timer).
func TestNewestActivity_ReportsNewestFileMtime(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	old := filepath.Join(ws, "old.txt")
	if err := os.WriteFile(old, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(old, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	newer := filepath.Join(ws, "newer.txt")
	if err := os.WriteFile(newer, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	newTime := time.Now().Add(-time.Minute)
	if err := os.Chtimes(newer, newTime, newTime); err != nil {
		t.Fatal(err)
	}
	// The observer's own sink: must be ignored even though it is the most
	// recent file on disk.
	events := filepath.Join(ws, "build"+observerEventsSuffix)
	if err := os.WriteFile(events, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	o := New(Config{WorkspaceDir: ws}, &bytes.Buffer{})
	got := o.newestActivity()
	if !got.Equal(newTime) {
		t.Errorf("newestActivity = %v, want %v (newest non-events file)", got, newTime)
	}
}

// TestNewestActivity_SkipsUnreadableEntries covers the walk-error branch
// (the err != nil arm of the WalkFunc): an unreadable subdirectory is skipped
// rather than aborting the scan, so a sibling readable file is still seen.
func TestNewestActivity_SkipsUnreadableEntries(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("chmod 000 is ineffective for root")
	}
	t.Parallel()
	ws := t.TempDir()
	good := filepath.Join(ws, "good.txt")
	if err := os.WriteFile(good, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	goodTime := time.Now().Add(-time.Minute)
	if err := os.Chtimes(good, goodTime, goodTime); err != nil {
		t.Fatal(err)
	}
	// An unreadable subdir → Walk invokes the WalkFunc with err != nil for
	// its children, exercising the skip-on-error arm.
	bad := filepath.Join(ws, "noread")
	if err := os.MkdirAll(filepath.Join(bad, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(bad, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bad, 0o755) }) // so t.TempDir cleanup can remove it

	o := New(Config{WorkspaceDir: ws}, &bytes.Buffer{})
	got := o.newestActivity()
	if got.IsZero() {
		t.Error("newestActivity returned zero time; expected the readable sibling to be counted despite the unreadable subdir")
	}
}

// TestNewestActivity_RespectsFileCap covers the activityScanMaxFiles backstop:
// once the cap is hit the walk short-circuits (filepath.SkipAll). We assert the
// scan still completes and returns a non-zero mtime — the cap must bound work,
// never abort the observer.
func TestNewestActivity_RespectsFileCap(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	for i := 0; i < activityScanMaxFiles+5; i++ {
		p := filepath.Join(ws, "f"+strconv.Itoa(i)+".txt")
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	o := New(Config{WorkspaceDir: ws}, &bytes.Buffer{})
	got := o.newestActivity()
	if got.IsZero() {
		t.Error("newestActivity returned zero time despite many files present; cap must bound work, not abort")
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
