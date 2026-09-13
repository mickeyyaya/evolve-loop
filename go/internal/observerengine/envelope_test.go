package observerengine

// envelope_test.go — §6 tests 28-29 and the G1 fold: the pure envelope
// builder replayed against the golden, the once-per-op append faults (marshal,
// mkdir, open, write — the write through the unexported openAppend seam,
// critic B1) and the INCIDENT retention point (after a successful open,
// regardless of the write result — the :559-563 quirk).

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func readGolden(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "<PID>", strconv.Itoa(fixturePID))
}

func goldenEngine(t *testing.T) *Engine {
	t.Helper()
	e, _, _ := newEngine(t, nil)
	e.eventCount, e.toolCallCount, e.toolResultCnt, e.errorCount, e.rateLimitCnt = 4, 1, 1, 1, 1
	e.cumulativeCost, e.cacheReadTok, e.cacheCreateTok = 0.5, 1024, 256
	return e
}

func goldenIncidentData() map[string]any {
	return map[string]any{"idle_s": 601, "threshold_s": 600, "action": "extend", "action_reason": "deep-thinking phase; extend"}
}

// TestEnvelope_GoldenReplay — G1 through the leaf with Settings.PID bound to
// the golden's <PID>. Kills M15 in the leaf.
func TestEnvelope_GoldenReplay(t *testing.T) {
	t.Parallel()
	e := goldenEngine(t)
	e.emit("Engine.Tick", "stuck_no_output", "INCIDENT", goldenIncidentData())
	raw, err := os.ReadFile(e.s.Paths.Events)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(raw), readGolden(t, "envelope.golden.json"); got != want {
		t.Fatalf("envelope:\n got: %s\nwant: %s", got, want)
	}
}

// TestEmit_IncidentAppendsAndIsRetained — the intent of the host's
// TestEmit_AppendsToEventsFile (phaseobserver_test.go:245, moved): an INCIDENT
// emit appends one parseable line and is retained; an INFO is not retained.
func TestEmit_IncidentAppendsAndIsRetained(t *testing.T) {
	t.Parallel()
	e, rc, _ := newEngine(t, nil)
	e.emit("Engine.Tick", "test_event", "INCIDENT", map[string]any{"foo": "bar"})
	e.emit("Engine.Tick", "heartbeat", "INFO", map[string]any{})
	events := readEvents(t, e.s.Paths.Events)
	if len(events) != 2 || events[0]["type"] != "test_event" || events[0]["severity"] != "INCIDENT" {
		t.Fatalf("events: %v", events)
	}
	if len(e.incidents) != 1 || e.incidents[0]["type"] != "test_event" {
		t.Errorf("INCIDENT retained, INFO not: %v", e.incidents)
	}
	if len(rc.all()) != 0 {
		t.Errorf("a successful append signals nothing: %+v", rc.all())
	}
}

type failingWriter struct{ closed int }

func (w *failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write: no space left on device")
}
func (w *failingWriter) Close() error { w.closed++; return nil }

// TestEmit_FaultsReportOncePerOp — each op reported exactly once across three
// emits; retention follows a successful open only. Kills M27 (once-guard),
// M43 (retention moved before the open), M44 (a swallowed write error).
func TestEmit_FaultsReportOncePerOp(t *testing.T) {
	t.Parallel()
	t.Run("marshal", func(t *testing.T) {
		t.Parallel()
		e, rc, _ := newEngine(t, nil)
		for i := 0; i < 3; i++ {
			e.emit("Engine.Start", "observer_started", "INCIDENT", map[string]any{"ch": make(chan int)})
		}
		assertOneFault(t, rc, "marshal", "Engine.Start", "observer_started", e.s.Paths.Events)
		if len(e.incidents) != 0 {
			t.Error("nothing is retained when the envelope cannot be marshalled")
		}
	})
	t.Run("mkdir", func(t *testing.T) {
		t.Parallel()
		file := filepath.Join(t.TempDir(), "ws-is-a-file")
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		e, rc, _ := newEngine(t, func(s *Settings, _ *Deps) { s.Paths = PathsFor(filepath.Join(file, "nested"), "builder") })
		for i := 0; i < 3; i++ {
			e.emit("Engine.Tick", "stuck_no_output", "INCIDENT", map[string]any{})
		}
		assertOneFault(t, rc, "mkdir", "Engine.Tick", "stuck_no_output", e.s.Paths.Events)
		if len(e.incidents) != 0 {
			t.Error("nothing is retained when the directory cannot be created")
		}
	})
	t.Run("open", func(t *testing.T) {
		t.Parallel()
		e, rc, _ := newEngine(t, nil)
		if err := os.Mkdir(e.s.Paths.Events, 0o755); err != nil { // a DIRECTORY at the events path
			t.Fatal(err)
		}
		for i := 0; i < 3; i++ {
			e.emit("Engine.Shutdown", "observer_shutdown", "INCIDENT", map[string]any{})
		}
		assertOneFault(t, rc, "open", "Engine.Shutdown", "observer_shutdown", e.s.Paths.Events)
		if len(e.incidents) != 0 {
			t.Error("the INCIDENT is NOT retained when the open fails (the :559-563 point is after the open)")
		}
	})
	t.Run("write", func(t *testing.T) {
		t.Parallel()
		e, rc, _ := newEngine(t, nil)
		w := &failingWriter{}
		e.openAppend = func(string) (io.WriteCloser, error) { return w, nil }
		for i := 0; i < 3; i++ {
			e.emit("Engine.Tick", "stuck_no_progress", "INCIDENT", map[string]any{})
		}
		assertOneFault(t, rc, "write", "Engine.Tick", "stuck_no_progress", e.s.Paths.Events)
		if len(e.incidents) != 3 {
			t.Errorf("the INCIDENT IS retained when only the write fails (report.incidents may list an INCIDENT whose line was lost): %d", len(e.incidents))
		}
		if w.closed != 3 {
			t.Errorf("the handle is closed after every attempt: %d", w.closed)
		}
	})
}

func assertOneFault(t *testing.T, rc *recordingCenter, op, origin, eventType, path string) {
	t.Helper()
	got := rc.byCode(CodeEventAppendFailed)
	if len(got) != 1 {
		t.Fatalf("op=%s: exactly one fault over three emits, got %d: %+v", op, len(got), got)
	}
	f := got[0].Fields
	if f["op"] != op || f["step"] != "emit" || f["event_type"] != eventType || f["path"] != path || got[0].Origin != origin || got[0].Reason == "" {
		t.Errorf("op=%s: %+v", op, got[0])
	}
}
