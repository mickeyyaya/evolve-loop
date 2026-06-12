package flock

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestLockHappyPathAndRelease(t *testing.T) {
	release, err := Lock(filepath.Join(t.TempDir(), "state.lock"))
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	release()
}

func TestLockMkdirError(t *testing.T) {
	base := t.TempDir()
	notDir := filepath.Join(base, "not-dir")
	if err := os.WriteFile(notDir, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Lock(filepath.Join(notDir, "state.lock")); err == nil {
		t.Fatal("Lock under a file path must fail while creating the parent directory")
	}
}

func TestLockOpenError(t *testing.T) {
	if _, err := Lock(t.TempDir()); err == nil {
		t.Fatal("Lock on a directory path must fail while opening the lock file")
	}
}

func TestLockFlockErrorClosesFile(t *testing.T) {
	old := flockFn
	t.Cleanup(func() { flockFn = old })
	want := errors.New("lock refused")
	flockFn = func(fd int, how int) error {
		if how != syscall.LOCK_EX {
			t.Fatalf("flockFn called with how=%d, want LOCK_EX", how)
		}
		return want
	}
	if _, err := Lock(filepath.Join(t.TempDir(), "state.lock")); err == nil {
		t.Fatal("Lock must surface flockFn LOCK_EX errors")
	}
}

func TestLockReleaseUnlocks(t *testing.T) {
	old := flockFn
	t.Cleanup(func() { flockFn = old })
	var calls []int
	flockFn = func(fd int, how int) error {
		calls = append(calls, how)
		return nil
	}
	release, err := Lock(filepath.Join(t.TempDir(), "state.lock"))
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	release()
	if len(calls) != 2 || calls[0] != syscall.LOCK_EX || calls[1] != syscall.LOCK_UN {
		t.Fatalf("flock calls = %v, want [LOCK_EX LOCK_UN]", calls)
	}
}
