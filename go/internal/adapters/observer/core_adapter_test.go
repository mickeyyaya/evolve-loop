package observer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// syncSink is a goroutine-safe wrapper around bytes.Buffer for tests.
// bytes.Buffer's Write + String race when accessed from the observer
// goroutine + test goroutine concurrently; syncSink serializes them
// behind one mutex.
type syncSink struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncSink) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

var _ io.Writer = (*syncSink)(nil)

// TestCoreAdapter_Start_ReturnsNoopCancelOnEmptyWorkspace pins the
// guard at the top of Start: pre-cycle / non-phase calls must not
// touch the filesystem and must return a non-nil cancel.
func TestCoreAdapter_Start_ReturnsNoopCancelOnEmptyWorkspace(t *testing.T) {
	t.Parallel()
	a := NewCoreAdapter()
	cancel := a.Start(context.Background(), "", core.PhaseRequest{})
	if cancel == nil {
		t.Fatal("expected non-nil cancel even for empty workspace+phase")
	}
	cancel() // must not panic
	cancel() // must be idempotent
}

// TestCoreAdapter_Start_CreatesEventsFile pins that a real Start opens
// (creates) the <workspace>/<phase>-observer-events.ndjson file so
// downstream tooling can read live events.
func TestCoreAdapter_Start_CreatesEventsFile(t *testing.T) {
	ws := t.TempDir()
	a := NewCoreAdapter(fastObserverPolicy())
	cancel := a.Start(context.Background(), "tdd", core.PhaseRequest{Workspace: ws, Cycle: 123})
	defer cancel()

	eventsPath := filepath.Join(ws, "tdd-observer-events.ndjson")
	// Give the goroutine a moment to write the "started" event.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(eventsPath); err == nil && info.Size() > 0 {
			return // success
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("expected %s to be created with events within 2s", eventsPath)
}

// TestCoreAdapter_Start_EmitsStallEventWhenFileNeverGrows is the
// cycle-122 shape regression test: workspace exists but the phase's
// stdout-log never appears (codex hung at modal). The observer must
// emit a stall_no_output INCIDENT.
func TestCoreAdapter_Start_EmitsStallEventWhenFileNeverGrows(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	sink := &syncSink{}
	a := &CoreAdapter{
		Sink:   sink,
		Config: fastObserverPolicy(),
	}
	cancel := a.Start(context.Background(), "tdd", core.PhaseRequest{Workspace: ws, Cycle: 123})

	// Wait deterministically until we see the stall event (or fail at deadline).
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(sink.String(), `"stall_no_output"`) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()

	out := sink.String()
	if !strings.Contains(out, `"stall_no_output"`) {
		t.Fatalf("expected stall_no_output event when log never created; got:\n%s", out)
	}
	if !strings.Contains(out, `"incident"`) {
		t.Errorf("stall event should be severity=incident; got:\n%s", out)
	}
	if !strings.Contains(out, `"cycle":123`) || !strings.Contains(out, `"phase":"tdd"`) {
		t.Errorf("event should carry cycle/phase attribution; got:\n%s", out)
	}
}

// TestCoreAdapter_Start_StopsCleanlyOnCancel pins the cleanup contract:
// calling the returned cancel function makes the watcher goroutine
// exit within the bounded wait (10s) and the events file gets a
// "stopped" entry.
func TestCoreAdapter_Start_StopsCleanlyOnCancel(t *testing.T) {
	sink := &syncSink{}
	a := &CoreAdapter{Sink: sink, Config: fastObserverPolicy()}

	cancel := a.Start(context.Background(), "scout", core.PhaseRequest{
		Workspace: t.TempDir(), Cycle: 7,
	})
	// Wait for "started"
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(sink.String(), `"started"`) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	start := time.Now()
	cancel()
	if elapsed := time.Since(start); elapsed > 10500*time.Millisecond {
		t.Errorf("cancel took %v, want <10.5s", elapsed)
	}

	out := sink.String()
	if !strings.Contains(out, `"stopped"`) {
		t.Errorf("expected stopped event after cancel; got:\n%s", out)
	}
}

// TestCoreAdapter_Start_ConcurrentSamePhase_IsIsolatedSafe pins
// thread-safety: two phases starting in parallel (unusual but possible
// under multi-execute) get isolated sinks + cancels.
func TestCoreAdapter_Start_ConcurrentSamePhase_IsIsolatedSafe(t *testing.T) {
	a := NewCoreAdapter(fastObserverPolicy())
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ws := t.TempDir()
			cancel := a.Start(context.Background(), "scout", core.PhaseRequest{Workspace: ws, Cycle: 1})
			defer cancel()
			// Brief work simulation.
			time.Sleep(50 * time.Millisecond)
		}()
	}
	wg.Wait()
	// Test passes if no race detected by -race + no panic.
}

func fastObserverPolicy() policy.ObserverPolicy {
	stallS, pollS := 1, 1
	return policy.ObserverPolicy{StallS: &stallS, PollS: &pollS}
}

// TestCoreAdapter_Start_DegradesToNoopWhenEventsFileUnopenable covers the
// os.OpenFile error branch in Start: when Sink is nil and the events file
// cannot be created, the adapter must degrade to a no-op cancel (never block
// the phase, per ADR-0030) rather than panic. We force the failure by pointing
// Workspace at a path that is a regular FILE, so the events path's parent is
// not a directory and OpenFile fails.
func TestCoreAdapter_Start_DegradesToNoopWhenEventsFileUnopenable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wsFile := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(wsFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &CoreAdapter{} // Sink nil → adapter tries to open the events file
	cancel := a.Start(context.Background(), "tdd", core.PhaseRequest{Workspace: wsFile, Cycle: 1})
	if cancel == nil {
		t.Fatal("expected non-nil (no-op) cancel when events file cannot be opened")
	}
	cancel() // must not panic
	cancel() // idempotent

	// No events file should have been created under the bogus workspace path.
	if _, err := os.Stat(filepath.Join(wsFile, "tdd-observer-events.ndjson")); err == nil {
		t.Error("events file unexpectedly created under a non-directory workspace")
	}
}

// helper to consume NDJSON if any future test needs it.
func decodeEvents(t *testing.T, body []byte) []Event {
	t.Helper()
	var out []Event
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		if line == "" {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("decode %q: %v", line, err)
		}
		out = append(out, e)
	}
	return out
}

// Ensure decodeEvents stays referenced (it's a useful helper future
// tests will lean on; this prevents 'unused' lint failure).
var _ = decodeEvents
