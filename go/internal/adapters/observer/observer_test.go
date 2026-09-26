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

// assertWatchActivityResetsStallTimer runs Watch on a virtual clock and writes
// the activity after the first idle sample. The next poll crosses the original
// deadline, so only a stall-timer reset prevents stall_no_output; no writer
// goroutine races the -race scheduler.
func assertWatchActivityResetsStallTimer(t *testing.T, cfg Config, activity func() error) {
	t.Helper()
	const stall = 200 * time.Millisecond
	base := time.Unix(1_700_000_000, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	started := false
	baselineSet := false
	clockChecks := 0
	var activityErr error
	cfg.StallS = stall
	cfg.PollS = time.Millisecond
	cfg.OnEvent = func(event Event) {
		if event.Type == "started" {
			started = true
		}
	}

	var sink bytes.Buffer
	observer := New(cfg, &sink)
	observer.nowFunc = func() time.Time {
		if !started {
			return base // timestamp for the initial started event
		}
		if !baselineSet {
			baselineSet = true
			return base // initial lastGrowth
		}
		clockChecks++
		switch clockChecks {
		case 1:
			activityErr = activity()
			return base.Add(150 * time.Millisecond)
		case 2:
			return base.Add(210 * time.Millisecond)
		default:
			cancel()
			return base.Add(300 * time.Millisecond)
		}
	}

	err := observer.Watch(ctx)
	if activityErr != nil {
		t.Fatalf("write activity: %v", activityErr)
	}
	if err != context.Canceled {
		t.Fatalf("Watch returned %v, want controlled context cancellation", err)
	}
	if clockChecks < 3 {
		t.Fatalf("clock checks = %d, want at least 3 to cross the original stall deadline", clockChecks)
	}
	for _, event := range parseEvents(t, sink.Bytes()) {
		if event.Type == "stall_no_output" {
			t.Errorf("stall fired despite observed activity resetting the timer: %+v", event)
		}
	}
}

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

func TestWatch_StallEmitsIncident(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "scout-stdout.log")
	// No growth after this write.
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

func TestWatch_GrowthResetsStallTimer(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "scout-stdout.log")
	if err := os.WriteFile(logFile, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertWatchActivityResetsStallTimer(t, Config{
		Cycle: 1, Phase: "scout", Agent: "x",
		StdoutLog: logFile,
	}, func() error {
		file, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		if _, err := file.Write([]byte("more ")); err != nil {
			_ = file.Close()
			return err
		}
		return file.Close()
	})
}

func TestWatch_LivenessProbeSuppressesFalseStall(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "tdd-stdout.log")
	// A flat log and no workspace: the window of one long silent turn.
	if err := os.WriteFile(logFile, []byte("start"), 0o644); err != nil {
		t.Fatal(err)
	}
	var sink syncBuffer
	o := New(Config{
		StallS:        60 * time.Millisecond,
		PollS:         10 * time.Millisecond,
		Cycle:         190,
		Phase:         "tdd",
		Agent:         "tdd",
		StdoutLog:     logFile,
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

func TestWatch_WorkspaceActivityResetsStallTimer(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "build-stdout.log")
	if err := os.WriteFile(logFile, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertWatchActivityResetsStallTimer(t, Config{
		Cycle: 1, Phase: "build", Agent: "build",
		StdoutLog:    logFile,
		WorkspaceDir: tmp,
	}, func() error {
		path := filepath.Join(tmp, "build-reflection.yaml")
		if err := os.WriteFile(path, []byte("line\n"), 0o644); err != nil {
			return err
		}
		future := time.Now().Add(time.Hour)
		return os.Chtimes(path, future, future)
	})
}

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

// awaitEvent waits for an event of eventType on ch. The timeout is a generous
// safety bound, not a tuned window, so a slow -race runner cannot flake it.
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

func TestWatch_ObserverEventsFileDoesNotMaskStall(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "build-stdout.log")
	if err := os.WriteFile(logFile, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}
	eventsFile := filepath.Join(tmp, "build-observer-events.ndjson")

	events := make(chan Event, 64)
	var sink syncBuffer
	o := New(Config{
		StallS: 40 * time.Millisecond, PollS: 5 * time.Millisecond,
		Cycle: 3, Phase: "build", Agent: "build",
		StdoutLog: logFile, WorkspaceDir: tmp,
		// Non-blocking, as OnEvent requires.
		OnEvent: func(e Event) {
			select {
			case events <- e:
			default:
			}
		},
	}, &sink)

	// Keep the observer's own events file growing throughout.
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

	if !awaitEvent(t, events, "stall_no_output", 5*time.Second) {
		t.Error("stall must fire even though the observer's own events file keeps growing (it must be excluded from the activity scan)")
	}
}

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

// The only assertion is that Watch does not panic on a missing log.
func TestWatch_MissingLogFile(t *testing.T) {
	t.Parallel()
	o := New(Config{
		StallS: 50 * time.Millisecond, PollS: 10 * time.Millisecond,
		Cycle: 1, Phase: "x", Agent: "x",
		StdoutLog: "/no/such/log",
	}, &bytes.Buffer{})
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_ = o.Watch(ctx)
}

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

func TestStatSize_EmptyStdoutLogReturnsZero(t *testing.T) {
	t.Parallel()
	o := New(Config{StdoutLog: ""}, &bytes.Buffer{})
	if got := o.statSize(); got != 0 {
		t.Errorf("statSize with empty StdoutLog = %d, want 0", got)
	}
}

func TestStatSize_MissingFileReturnsZero(t *testing.T) {
	t.Parallel()
	o := New(Config{StdoutLog: filepath.Join(t.TempDir(), "never-created.log")}, &bytes.Buffer{})
	if got := o.statSize(); got != 0 {
		t.Errorf("statSize on missing file = %d, want 0", got)
	}
}

func TestNewestActivity_UnsetWorkspaceReturnsZeroTime(t *testing.T) {
	t.Parallel()
	o := New(Config{WorkspaceDir: ""}, &bytes.Buffer{})
	if got := o.newestActivity(); !got.IsZero() {
		t.Errorf("newestActivity with unset WorkspaceDir = %v, want zero time", got)
	}
}

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
	// The newest file on disk, but it is the observer's own sink.
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
	// An unreadable subdir makes Walk report an error for its children.
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

// syncBuffer is a mutex-guarded bytes.Buffer: the watcher goroutine writes while the test reads.
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
