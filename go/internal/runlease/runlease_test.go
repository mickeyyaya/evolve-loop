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

// TestOwnerLive_* — cycle-554 workspace-hygiene-s1 RED contract. Today
// liveness is freshness-only (10min lease TTL), so a crashed owner with a
// still-fresh heartbeat (the common 2-6min post-crash window) reads as "live"
// and blocks SealCycle/the loop guard, forcing `evolve cycle reset --force` at
// every batch boundary (plan docs/plans/workspace-hygiene-2026-07.md §S1).
// OwnerLive adds a pid-liveness probe on top of Fresh: dead pid + fresh
// heartbeat ⇒ not live (the fix); live pid + fresh heartbeat ⇒ still live (the
// invariant the fence exists for, unchanged).

func TestOwnerLive_DeadPidFreshHeartbeatNotLive(t *testing.T) {
	l := Lease{OwnerPID: 4242, HeartbeatAt: t0.UTC().Format(time.RFC3339Nano)}
	alive := func(pid int) bool { return false } // owner process is gone
	if OwnerLive(l, t0, 0, alive) {
		t.Error("fresh heartbeat with a dead owner pid must NOT be live")
	}
}

func TestOwnerLive_FreshAliveOwnerIsLive(t *testing.T) {
	l := Lease{OwnerPID: 4242, HeartbeatAt: t0.UTC().Format(time.RFC3339Nano)}
	alive := func(pid int) bool { return pid == 4242 }
	if !OwnerLive(l, t0, 0, alive) {
		t.Error("fresh heartbeat with a genuinely live owner pid must be live (safety invariant unchanged)")
	}
}

func TestOwnerLive_StaleAliveOwnerNotLive(t *testing.T) {
	l := Lease{OwnerPID: 4242, HeartbeatAt: t0.UTC().Format(time.RFC3339Nano)}
	alive := func(pid int) bool { return true }
	if OwnerLive(l, t0.Add(20*time.Minute), 0, alive) {
		t.Error("a stale heartbeat must never be live, even when the pid happens to be alive (pid reuse case)")
	}
}

// TestOwnerLive_NilAlive — EDGE/back-compat: a caller that passes no probe
// keeps the old freshness-only behavior; no probe call, no panic.
func TestOwnerLive_NilAlive(t *testing.T) {
	l := Lease{OwnerPID: 4242, HeartbeatAt: t0.UTC().Format(time.RFC3339Nano)}
	if !OwnerLive(l, t0, 0, nil) {
		t.Error("nil alive probe must fall back to freshness-only: fresh heartbeat must be live")
	}
	if OwnerLive(l, t0.Add(20*time.Minute), 0, nil) {
		t.Error("nil alive probe must fall back to freshness-only: stale heartbeat must not be live")
	}
}

// TestOwnerLive_Pid0 — EDGE/back-compat: leases written before pid tracking
// existed (or by a caller that never sets OwnerPID) must resolve via Fresh
// alone; the probe must not even be consulted.
func TestOwnerLive_Pid0(t *testing.T) {
	l := Lease{OwnerPID: 0, HeartbeatAt: t0.UTC().Format(time.RFC3339Nano)}
	called := false
	alive := func(int) bool { called = true; return false }
	if !OwnerLive(l, t0, 0, alive) {
		t.Error("OwnerPID==0 must fall back to freshness-only regardless of what alive would report")
	}
	if called {
		t.Error("alive must not be consulted when OwnerPID==0")
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
	t.Parallel()
	tmpPath := filepath.Join(t.TempDir(), ".lease.test.tmp")
	if err := os.WriteFile(tmpPath, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := writer{
		createTemp: func(dir, pattern string) (tempFile, error) {
			return failingTempFile{name: tmpPath, writeErr: errors.New("disk full")}, nil
		},
		rename: os.Rename,
	}
	if err := w.write(t.TempDir(), Lease{RunID: "r"}, t0); err == nil {
		t.Fatal("write must surface temp-file write errors")
	}
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatalf("write must remove temp file after write error, stat err=%v", err)
	}
}

func TestWrite_CloseErrorRemovesTemp(t *testing.T) {
	t.Parallel()
	tmpPath := filepath.Join(t.TempDir(), ".lease.test.tmp")
	if err := os.WriteFile(tmpPath, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := writer{
		createTemp: func(dir, pattern string) (tempFile, error) {
			return failingTempFile{name: tmpPath, closeErr: errors.New("close failed")}, nil
		},
		rename: os.Rename,
	}
	if err := w.write(t.TempDir(), Lease{RunID: "r"}, t0); err == nil {
		t.Fatal("write must surface temp-file close errors")
	}
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatalf("write must remove temp file after close error, stat err=%v", err)
	}
}

func TestWrite_RenameSeamErrorRemovesTemp(t *testing.T) {
	t.Parallel()
	tmpPath := filepath.Join(t.TempDir(), ".lease.test.tmp")
	if err := os.WriteFile(tmpPath, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := writer{
		createTemp: func(dir, pattern string) (tempFile, error) {
			return failingTempFile{name: tmpPath}, nil
		},
		rename: func(oldpath, newpath string) error {
			return errors.New("rename failed")
		},
	}
	if err := w.write(t.TempDir(), Lease{RunID: "r"}, t0); err == nil {
		t.Fatal("write must surface rename errors")
	}
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatalf("write must remove temp file after rename error, stat err=%v", err)
	}
}

// TestWriter_PerInstanceSeamsAreIsolated is the acceptance for moving the write
// seams off package globals (concurrency-hygiene): two writers with DIFFERENT
// injected seams operate independently. The old package-var form (one shared
// createTempFn/renameFn) structurally could not hold two live seams at once — so
// this proves the seams are per-instance, which is what lets the seam tests
// above run t.Parallel() without racing shared state.
func TestWriter_PerInstanceSeamsAreIsolated(t *testing.T) {
	t.Parallel()
	var aCreated, bRenamed bool
	a := writer{
		createTemp: func(dir, pattern string) (tempFile, error) { aCreated = true; return os.CreateTemp(dir, pattern) },
		rename:     os.Rename,
	}
	b := writer{
		createTemp: func(dir, pattern string) (tempFile, error) { return os.CreateTemp(dir, pattern) },
		rename:     func(oldpath, newpath string) error { bRenamed = true; return os.Rename(oldpath, newpath) },
	}
	if err := a.write(t.TempDir(), Lease{RunID: "a"}, t0); err != nil {
		t.Fatal(err)
	}
	if !aCreated {
		t.Error("writer a must use its own createTemp seam")
	}
	if bRenamed {
		t.Error("writer a must NOT touch writer b's seam (seams are per-instance)")
	}
	if err := b.write(t.TempDir(), Lease{RunID: "b"}, t0); err != nil {
		t.Fatal(err)
	}
	if !bRenamed {
		t.Error("writer b must use its own rename seam")
	}
}
