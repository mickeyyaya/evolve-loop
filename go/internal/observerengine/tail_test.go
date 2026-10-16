package observerengine

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func tailFaults(rc *recordingCenter) []signalcenter.Event { return rc.byCode(CodeStdoutTailFailed) }

func TestTail_StatMissReturnsPriorOffset(t *testing.T) {
	t.Parallel()
	e, rc, _ := newEngine(t, func(s *Settings, _ *Deps) { s.Paths.Stdout = filepath.Join(t.TempDir(), "does-not-exist.log") })
	e.lastByteOff = 42
	lines, off := e.tail()
	if lines != nil || off != 42 {
		t.Errorf("lines=%v off=%d, want nil/42 (prior offset preserved on stat miss)", lines, off)
	}
	e.tail()
	if len(tailFaults(rc)) != 0 {
		t.Errorf("ENOENT is silent by design (tmux drivers: the log appears at exit): %+v", rc.all())
	}
}

func TestTail_RotationResetsOffset(t *testing.T) {
	t.Parallel()
	e, _, ws := newEngine(t, nil)
	seedLog(t, filepath.Join(ws, "builder-stdout.log"), "line-after-rotation")
	e.lastByteOff = 9999
	lines, off := e.tail()
	if len(lines) != 1 || lines[0] != "line-after-rotation" {
		t.Fatalf("lines = %v, want [line-after-rotation] (rotation re-read from 0)", lines)
	}
	if off != int64(len("line-after-rotation\n")) || e.lastByteOff != 0 {
		t.Errorf("offset = %d, lastByteOff = %d", off, e.lastByteOff)
	}
}

func TestTail_StatNotDirReportsOnce(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	e, rc, _ := newEngine(t, func(s *Settings, _ *Deps) { s.Paths.Stdout = filepath.Join(file, "stdout.log") })
	e.lastByteOff = 7
	for i := 0; i < 3; i++ {
		if lines, off := e.tail(); lines != nil || off != 7 {
			t.Errorf("tick %d: lines=%v off=%d", i, lines, off)
		}
	}
	got := tailFaults(rc)
	if len(got) != 1 || got[0].Fields["op"] != "stat" || got[0].Fields["step"] != "tail" || got[0].Fields["path"] != e.s.Paths.Stdout || got[0].Origin != "Engine.Tick" {
		t.Errorf("one op=stat fault: %+v", got)
	}
}

func TestTail_OpenFailureReportsOnceAndKeepsOffset(t *testing.T) {
	t.Parallel()
	short, err := os.MkdirTemp("", "u12")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(short) })
	sock := filepath.Join(short, "s.sock")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	e, rc, _ := newEngine(t, func(s *Settings, _ *Deps) { s.Paths.Stdout = sock })
	e.lastByteOff = 0
	for i := 0; i < 3; i++ {
		if lines, off := e.tail(); lines != nil || off != 0 {
			t.Errorf("tick %d: lines=%v off=%d", i, lines, off)
		}
	}
	got := tailFaults(rc)
	if len(got) != 1 || got[0].Fields["op"] != "open" || got[0].Fields["step"] != "tail" || got[0].Fields["path"] != sock {
		t.Fatalf("exactly one op=open fault with step/path: %+v", got)
	}
}

func TestTail_SeekFailureReportsOnce(t *testing.T) {
	t.Parallel()
	e, rc, ws := newEngine(t, nil)
	seedLog(t, filepath.Join(ws, "builder-stdout.log"), `{"type":"result"}`)
	e.lastByteOff = -1
	for i := 0; i < 2; i++ {
		if lines, off := e.tail(); lines != nil || off != -1 {
			t.Errorf("tick %d: lines=%v off=%d", i, lines, off)
		}
	}
	got := tailFaults(rc)
	if len(got) != 1 || got[0].Fields["op"] != "seek" {
		t.Fatalf("one op=seek fault: %+v", got)
	}
	e.lastByteOff = 0
	lines, off := e.tail()
	if len(lines) != 1 || off != int64(len(`{"type":"result"}`)+1) {
		t.Errorf("a valid offset reads the line and returns the file size: %v %d", lines, off)
	}
}

func TestTail_OversizedLineReportsScanAndAdvances(t *testing.T) {
	e, rc, ws := newEngine(t, nil)
	big := strings.Repeat("x", 11*1024*1024)
	path := filepath.Join(ws, "builder-stdout.log")
	if err := os.WriteFile(path, []byte(`{"type":"result"}`+"\n"+big), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, off := e.tail()
	if len(lines) != 1 || off != int64(len(`{"type":"result"}`)+1+len(big)) {
		t.Errorf("the complete line before the oversized one is returned and the offset is the size: %d lines, off %d", len(lines), off)
	}
	e.tail()
	got := tailFaults(rc)
	if len(got) != 1 || got[0].Fields["op"] != "scan" {
		t.Fatalf("one op=scan fault: %+v", got)
	}
}

func TestTail_UnterminatedLastLineIsLostAtThisOffset(t *testing.T) {
	t.Parallel()
	e, rc, ws := newEngine(t, nil)
	path := filepath.Join(ws, "builder-stdout.log")
	if err := os.WriteFile(path, []byte(`{"type":"assis`), 0o644); err != nil {
		t.Fatal(err)
	}
	e.ingest()
	if e.eventCount != 0 || e.lastByteOff != int64(len(`{"type":"assis`)) {
		t.Errorf("the partial line is skipped and the offset advanced past it: events=%d off=%d", e.eventCount, e.lastByteOff)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`tant","message":{"content":[]}}` + "\n")
	_ = f.Close()
	e.ingest()
	if e.eventCount != 0 {
		t.Errorf("the completed remainder is malformed on its own and skipped: events=%d", e.eventCount)
	}
	if len(tailFaults(rc)) != 0 {
		t.Errorf("no fault on the quirk path: %+v", rc.all())
	}
}
