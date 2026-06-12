package runlease

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var t0 = time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)

type failingTempFile struct {
	name     string
	writeErr error
	closeErr error
}

func (f failingTempFile) Name() string {
	return f.name
}

func (f failingTempFile) Write([]byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return 1, nil
}

func (f failingTempFile) Close() error {
	return f.closeErr
}

func TestWriteReadFresh_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := Write(dir, Lease{RunID: "01JTEST", OwnerPID: 123}, t0); err != nil {
		t.Fatalf("Write: %v", err)
	}
	l, ok, err := Read(dir)
	if err != nil || !ok {
		t.Fatalf("Read: ok=%v err=%v", ok, err)
	}
	if l.RunID != "01JTEST" || l.OwnerPID != 123 {
		t.Errorf("round-trip mismatch: %+v", l)
	}
	if !Fresh(l, t0.Add(5*time.Minute), 0) {
		t.Error("5min-old heartbeat must be fresh within the 10min default TTL")
	}
	if Fresh(l, t0.Add(11*time.Minute), 0) {
		t.Error("11min-old heartbeat must be stale past the 10min default TTL")
	}
	if !Fresh(l, t0.Add(20*time.Minute), 30*time.Minute) {
		t.Error("explicit TTL must override the default")
	}
}

func TestRead_AbsentIsNotAnError(t *testing.T) {
	_, ok, err := Read(t.TempDir())
	if err != nil || ok {
		t.Fatalf("absent lease: want ok=false err=nil, got ok=%v err=%v", ok, err)
	}
}

func TestRead_GarbageIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("{torn"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Read(dir); err == nil {
		t.Fatal("unparsable lease must surface an error (caller decides; never silently live/dead)")
	}
}

func TestRead_FileError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(PathIn(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Read(dir); err == nil {
		t.Fatal("lease read errors other than not-exist must be surfaced")
	}
}

func TestFresh_UnparsableTimestampNeverFresh(t *testing.T) {
	if Fresh(Lease{HeartbeatAt: "not-a-time"}, t0, 0) {
		t.Error("garbage heartbeat must never prove liveness")
	}
}

func TestWrite_RefreshesHeartbeat(t *testing.T) {
	dir := t.TempDir()
	if err := Write(dir, Lease{RunID: "r"}, t0); err != nil {
		t.Fatal(err)
	}
	if err := Write(dir, Lease{RunID: "r"}, t0.Add(9*time.Minute)); err != nil {
		t.Fatal(err)
	}
	l, _, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !Fresh(l, t0.Add(15*time.Minute), 0) {
		t.Error("refreshed lease must be fresh relative to the new heartbeat")
	}
}

func TestWrite_CreateTempError(t *testing.T) {
	base := t.TempDir()
	notDir := filepath.Join(base, "not-dir")
	if err := os.WriteFile(notDir, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(notDir, Lease{RunID: "r"}, t0); err == nil {
		t.Fatal("Write must fail when the run directory is not a directory")
	}
}

func TestWrite_RenameErrorRemovesTemp(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(PathIn(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Write(dir, Lease{RunID: "r"}, t0); err == nil {
		t.Fatal("Write must fail when the destination lease path is a directory")
	}
	matches, err := filepath.Glob(filepath.Join(dir, FileName+".*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("Write left temp files after rename failure: %v", matches)
	}
}

func TestWrite_WriteErrorRemovesTemp(t *testing.T) {
	old := createTempFn
	t.Cleanup(func() { createTempFn = old })
	tmpPath := filepath.Join(t.TempDir(), ".lease.test.tmp")
	if err := os.WriteFile(tmpPath, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	createTempFn = func(dir, pattern string) (tempFile, error) {
		return failingTempFile{name: tmpPath, writeErr: errors.New("disk full")}, nil
	}
	if err := Write(t.TempDir(), Lease{RunID: "r"}, t0); err == nil {
		t.Fatal("Write must surface temp-file write errors")
	}
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatalf("Write must remove temp file after write error, stat err=%v", err)
	}
}

func TestWrite_CloseErrorRemovesTemp(t *testing.T) {
	old := createTempFn
	t.Cleanup(func() { createTempFn = old })
	tmpPath := filepath.Join(t.TempDir(), ".lease.test.tmp")
	if err := os.WriteFile(tmpPath, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	createTempFn = func(dir, pattern string) (tempFile, error) {
		return failingTempFile{name: tmpPath, closeErr: errors.New("close failed")}, nil
	}
	if err := Write(t.TempDir(), Lease{RunID: "r"}, t0); err == nil {
		t.Fatal("Write must surface temp-file close errors")
	}
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatalf("Write must remove temp file after close error, stat err=%v", err)
	}
}

func TestWrite_RenameSeamErrorRemovesTemp(t *testing.T) {
	oldCreateTemp := createTempFn
	oldRename := renameFn
	t.Cleanup(func() {
		createTempFn = oldCreateTemp
		renameFn = oldRename
	})
	tmpPath := filepath.Join(t.TempDir(), ".lease.test.tmp")
	if err := os.WriteFile(tmpPath, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	createTempFn = func(dir, pattern string) (tempFile, error) {
		return failingTempFile{name: tmpPath}, nil
	}
	renameFn = func(oldpath, newpath string) error {
		return errors.New("rename failed")
	}
	if err := Write(t.TempDir(), Lease{RunID: "r"}, t0); err == nil {
		t.Fatal("Write must surface rename errors")
	}
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatalf("Write must remove temp file after rename error, stat err=%v", err)
	}
}
