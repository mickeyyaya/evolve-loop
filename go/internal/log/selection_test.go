package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

var testSelection = Selection{Run: "r1", Cycle: 7, Phase: "build", Dispatch: "r1/7/build/1", Cat: "loop", Component: "log", Code: "LOG_X"}

func TestSelectionLogger_JSONCarriesNineKeys(t *testing.T) {
	var j, tx bytes.Buffer
	l := NewSelectionLogger(Sinks{JSON: &j, Text: &tx, Level: "info"})
	l.InfoContext(WithSelection(context.Background(), testSelection), "hello")
	var rec map[string]any
	if err := json.Unmarshal(j.Bytes(), &rec); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"ts", "run", "cycle", "phase", "dispatch", "cat", "component", "level", "code"} {
		if _, ok := rec[k]; !ok {
			t.Errorf("missing %s: %v", k, rec)
		}
	}
	if !strings.Contains(tx.String(), "component=log") {
		t.Errorf("text lacks component: %q", tx.String())
	}
}

func TestSelectionLogger_RecordAttrWinsOnce(t *testing.T) {
	var j bytes.Buffer
	l := NewSelectionLogger(Sinks{JSON: &j, Level: "info"})
	l.InfoContext(WithSelection(context.Background(), testSelection), "x", slog.String("code", "OVR"))
	if strings.Count(j.String(), `"code"`) != 1 || !strings.Contains(j.String(), "OVR") {
		t.Errorf("got %s", j.String())
	}
}

func TestSelectionLogger_LevelFiltersAndWithAttrsGroup(t *testing.T) {
	var j, tx bytes.Buffer
	l := NewSelectionLogger(Sinks{JSON: &j, Text: &tx, Level: "warn"})
	l.Info("dropped")
	if j.Len() != 0 || tx.Len() != 0 {
		t.Fatal("info passed warn level")
	}
	l.With("a", "b").WithGroup("g").Warn("kept", "k", "v")
	if !strings.Contains(j.String(), `"a":"b"`) || !strings.Contains(tx.String(), "a=b") {
		t.Errorf("json=%q text=%q", j.String(), tx.String())
	}
}

func TestSelectionLogger_RenderWritesOneLine(t *testing.T) {
	var tx bytes.Buffer
	l := NewSelectionLogger(Sinks{Text: &tx, Level: "debug", Render: func(r slog.Record) string { return "L:" + r.Message }})
	l.With("a", "b").WithGroup("g").Info("m")
	l.Debug("d")
	if tx.String() != "L:m\nL:d\n" {
		t.Errorf("got %q", tx.String())
	}
}

type stubHandler struct {
	level slog.Level
	err   error
	calls int
}

func (s *stubHandler) Enabled(_ context.Context, l slog.Level) bool { return l >= s.level }
func (s *stubHandler) Handle(context.Context, slog.Record) error    { s.calls++; return s.err }
func (s *stubHandler) WithAttrs([]slog.Attr) slog.Handler           { return s }
func (s *stubHandler) WithGroup(string) slog.Handler                { return s }

func TestSelectionHandler_SkipsDisabledAndReturnsFirstError(t *testing.T) {
	off := &stubHandler{level: slog.LevelError}
	failA := &stubHandler{level: slog.LevelInfo, err: errors.New("a")}
	failB := &stubHandler{level: slog.LevelInfo, err: errors.New("b")}
	h := selectionHandler{handlers: []slog.Handler{off, failA, failB}}
	err := h.Handle(context.Background(), slog.NewRecord(time.Time{}, slog.LevelInfo, "m", 0))
	if err == nil || err.Error() != "a" {
		t.Fatalf("want first error a, got %v", err)
	}
	if off.calls != 0 || failA.calls != 1 || failB.calls != 1 {
		t.Errorf("calls off=%d a=%d b=%d", off.calls, failA.calls, failB.calls)
	}
}
