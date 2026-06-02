package atomicwrite

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withSeams temporarily overrides the package seams and restores them on
// cleanup. Tests that call it mutate package-level state, so they must NOT call
// t.Parallel() (only the seam-free happy-path/marshal tests do).
func withSeams(t *testing.T, set func()) {
	t.Helper()
	origMkdir, origRename, origRemove, origCreate := mkdirAll, renameFile, removeFile, createTemp
	t.Cleanup(func() {
		mkdirAll, renameFile, removeFile, createTemp = origMkdir, origRename, origRemove, origCreate
	})
	set()
}

// fakeTemp is an injectable tempFile whose ops fail on demand.
type fakeTemp struct {
	name                         string
	writeErr, chmodErr, closeErr error
	closed                       bool
}

func (f *fakeTemp) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(p), nil
}
func (f *fakeTemp) Name() string            { return f.name }
func (f *fakeTemp) Chmod(fs.FileMode) error { return f.chmodErr }
func (f *fakeTemp) Close() error            { f.closed = true; return f.closeErr }

func TestBytes_HappyPath_WritesAtomicallyWith0644(t *testing.T) {
	t.Parallel()
	// Parent dir does not exist yet → also covers the MkdirAll path.
	path := filepath.Join(t.TempDir(), "sub", "out.txt")
	if err := Bytes(path, []byte("hello")); err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "hello" {
		t.Fatalf("read back = %q, %v; want hello", got, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644", info.Mode().Perm())
	}
	// No temp file left behind.
	entries, _ := os.ReadDir(filepath.Dir(path))
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("leaked temp file %s", e.Name())
		}
	}
}

func TestJSON_ProducesIndentedJSONNoTrailingNewline(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "v.json")
	if err := JSON(path, map[string]int{"a": 1}); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	got, _ := os.ReadFile(path)
	want := "{\n  \"a\": 1\n}"
	if string(got) != want {
		t.Fatalf("JSON bytes = %q, want %q", got, want)
	}
}

func TestJSON_MarshalError(t *testing.T) {
	t.Parallel()
	// A channel cannot be marshaled → exercises the marshal-error branch.
	err := JSON(filepath.Join(t.TempDir(), "x.json"), make(chan int))
	if err == nil || !strings.Contains(err.Error(), "marshal") {
		t.Fatalf("want marshal error, got %v", err)
	}
}

func TestBytes_MkdirError(t *testing.T) {
	sentinel := errors.New("mkdir boom")
	withSeams(t, func() { mkdirAll = func(string, fs.FileMode) error { return sentinel } })
	err := Bytes(filepath.Join(t.TempDir(), "a", "b"), []byte("x"))
	if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "mkdir") {
		t.Fatalf("want wrapped mkdir error, got %v", err)
	}
}

func TestBytes_CreateTempError(t *testing.T) {
	sentinel := errors.New("createtemp boom")
	withSeams(t, func() {
		createTemp = func(string, string) (tempFile, error) { return nil, sentinel }
	})
	err := Bytes(filepath.Join(t.TempDir(), "out"), []byte("x"))
	if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "create temp") {
		t.Fatalf("want wrapped create-temp error, got %v", err)
	}
}

func TestBytes_WriteError_CleansUpTemp(t *testing.T) {
	sentinel := errors.New("write boom")
	ft := &fakeTemp{name: filepath.Join(t.TempDir(), ".out.123.tmp"), writeErr: sentinel}
	var removed string
	withSeams(t, func() {
		createTemp = func(string, string) (tempFile, error) { return ft, nil }
		removeFile = func(n string) error { removed = n; return nil }
	})
	err := Bytes(filepath.Join(t.TempDir(), "out"), []byte("x"))
	if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "write") {
		t.Fatalf("want wrapped write error, got %v", err)
	}
	if !ft.closed || removed != ft.name {
		t.Errorf("expected temp closed (%v) and removed (%q==%q)", ft.closed, removed, ft.name)
	}
}

func TestBytes_ChmodError_CleansUpTemp(t *testing.T) {
	sentinel := errors.New("chmod boom")
	ft := &fakeTemp{name: "t.tmp", chmodErr: sentinel}
	var removed bool
	withSeams(t, func() {
		createTemp = func(string, string) (tempFile, error) { return ft, nil }
		removeFile = func(string) error { removed = true; return nil }
	})
	err := Bytes(filepath.Join(t.TempDir(), "out"), []byte("x"))
	if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "chmod") {
		t.Fatalf("want wrapped chmod error, got %v", err)
	}
	if !ft.closed || !removed {
		t.Error("expected temp closed and removed on chmod failure")
	}
}

func TestBytes_CloseError_CleansUpTemp(t *testing.T) {
	sentinel := errors.New("close boom")
	ft := &fakeTemp{name: "t.tmp", closeErr: sentinel}
	var removed bool
	withSeams(t, func() {
		createTemp = func(string, string) (tempFile, error) { return ft, nil }
		removeFile = func(string) error { removed = true; return nil }
	})
	err := Bytes(filepath.Join(t.TempDir(), "out"), []byte("x"))
	if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "close") {
		t.Fatalf("want wrapped close error, got %v", err)
	}
	if !removed {
		t.Error("expected temp removed on close failure")
	}
}

func TestBytes_RenameError_CleansUpTemp(t *testing.T) {
	sentinel := errors.New("rename boom")
	ft := &fakeTemp{name: "t.tmp"}
	var removed bool
	withSeams(t, func() {
		createTemp = func(string, string) (tempFile, error) { return ft, nil }
		renameFile = func(string, string) error { return sentinel }
		removeFile = func(string) error { removed = true; return nil }
	})
	err := Bytes(filepath.Join(t.TempDir(), "out"), []byte("x"))
	if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "rename") {
		t.Fatalf("want wrapped rename error, got %v", err)
	}
	if !removed {
		t.Error("expected temp removed on rename failure")
	}
}
