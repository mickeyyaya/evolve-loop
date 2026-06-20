package guardslog

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// failCloser accepts writes but fails Close — to prove Append surfaces the
// close error (the errcheck bug being fixed: the close error was discarded).
type failCloser struct{ closeErr error }

func (f failCloser) Write(p []byte) (int, error) { return len(p), nil }
func (f failCloser) Close() error                { return f.closeErr }

// writeFailCloser fails Write — to prove a write error takes precedence over
// (and is surfaced ahead of) the close error.
type writeFailCloser struct{ writeErr error }

func (w writeFailCloser) Write([]byte) (int, error) { return 0, w.writeErr }
func (w writeFailCloser) Close() error              { return errors.New("close also fails") }

func TestAppend_SurfacesCloseError(t *testing.T) {
	sentinel := errors.New("close boom")
	orig := openAppend
	openAppend = func(string) (io.WriteCloser, error) { return failCloser{closeErr: sentinel}, nil }
	defer func() { openAppend = orig }()

	err := Append(filepath.Join(t.TempDir(), "g.log"), "label", "msg", time.Unix(0, 0).UTC())
	if !errors.Is(err, sentinel) {
		t.Fatalf("Append must surface the close error, got %v", err)
	}
}

func TestAppend_WritesLine(t *testing.T) {
	// Real file (default openAppend), nested missing dir → MkdirAll creates it.
	path := filepath.Join(t.TempDir(), "sub", "guards.log")
	if err := Append(path, "commit-prefix-gate", "hello", time.Unix(0, 0).UTC()); err != nil {
		t.Fatalf("Append returned %v, want nil on a writable path", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	want := "[1970-01-01T00:00:00Z] [commit-prefix-gate] hello\n"
	if got := string(b); got != want {
		t.Errorf("line = %q, want %q", got, want)
	}
}

func TestAppend_EmptyPathNoop(t *testing.T) {
	if err := Append("", "label", "msg", time.Unix(0, 0).UTC()); err != nil {
		t.Errorf("empty path must be a nil-error no-op, got %v", err)
	}
}

func TestAppend_WriteErrorTakesPrecedence(t *testing.T) {
	sentinel := errors.New("write boom")
	orig := openAppend
	openAppend = func(string) (io.WriteCloser, error) { return writeFailCloser{writeErr: sentinel}, nil }
	defer func() { openAppend = orig }()

	if err := Append(filepath.Join(t.TempDir(), "g.log"), "label", "msg", time.Unix(0, 0).UTC()); !errors.Is(err, sentinel) {
		t.Fatalf("Append must surface the write error ahead of the close error, got %v", err)
	}
}

func TestAppend_MkdirErrorSurfaced(t *testing.T) {
	// Make the parent path a FILE so MkdirAll(filepath.Dir(path)) fails.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "notadir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Append(filepath.Join(blocker, "child.log"), "label", "msg", time.Unix(0, 0).UTC()); err == nil {
		t.Fatal("Append must surface the MkdirAll error when a parent is a file")
	}
}

func TestAppend_OpenErrorSurfaced(t *testing.T) {
	sentinel := errors.New("open boom")
	orig := openAppend
	openAppend = func(string) (io.WriteCloser, error) { return nil, sentinel }
	defer func() { openAppend = orig }()

	if err := Append(filepath.Join(t.TempDir(), "g.log"), "label", "msg", time.Unix(0, 0).UTC()); !errors.Is(err, sentinel) {
		t.Fatalf("Append must surface the open error, got %v", err)
	}
}
