package signalcenter

// coverage_test.go — the branches the behavior tests do not reach by
// themselves: option clamps, a listener removed by another during delivery,
// and the durable sink's failure paths (through the package-internal opener
// seam, so no real filesystem fault is needed).

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestWithRecentLimit_ClampsToOne(t *testing.T) {
	t.Parallel()
	c := New(WithRecentLimit(0), WithClock(nil))
	c.Emit(infoEvent("a"))
	c.Emit(infoEvent("b"))
	if r := c.Recent(); len(r) != 1 || r[0].Reason != "b" || r[0].TS == "" {
		t.Errorf("a limit below 1 clamps to 1 and a nil clock keeps the default: %+v", r)
	}
}

func TestDeliver_SkipsAListenerUnsubscribedByAnEarlierOne(t *testing.T) {
	t.Parallel()
	c := New()
	var unsubSecond func()
	first := 0
	c.Subscribe(func(Event) { first++; unsubSecond() })
	second := 0
	unsubSecond = c.Subscribe(func(Event) { second++ })
	c.Emit(infoEvent("x"))
	if first != 1 || second != 0 {
		t.Errorf("a listener unsubscribed during the same fan-out is not called: first=%d second=%d", first, second)
	}
}

type failingWriter struct{ err error }

func (f failingWriter) Write([]byte) (int, error) { return 0, f.err }
func (f failingWriter) Close() error              { return nil }

func TestNDJSONSink_OpenAndWriteFailuresCountAsDropped(t *testing.T) {
	t.Parallel()
	c := New()
	calls := 0
	s := newNDJSONSink(c, func(int) string { return "any" }, func(string) (io.WriteCloser, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("open refused")
		}
		return failingWriter{err: errors.New("disk full")}, nil
	})
	e := infoEvent("x")
	e.Cycle = 1
	s.write(e)
	s.write(e)
	if s.dropped != 2 {
		t.Errorf("an open failure and a write failure each count as a drop, got %d", s.dropped)
	}
	realRoot := t.TempDir()
	blocker := filepath.Join(realRoot, "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := openAppend(filepath.Join(blocker, "sub", "signals.ndjson")); err == nil {
		t.Error("the production opener reports a directory that cannot be created")
	}
	if _, err := openAppend(realRoot); err == nil {
		t.Error("the production opener reports a path that is a directory")
	}
}
