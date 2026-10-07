package flock_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

func TestLockShared_ReadersHoldItTogetherAndAWriterWaits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.lock")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := flock.LockShared(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.Open(path)
	if err != nil {
		first()
		t.Fatal(err)
	}
	defer second.Close()

	shareErr := syscall.Flock(int(second.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
	_, writerHeldOff, err := flock.TryLock(path)

	first()
	_ = syscall.Flock(int(second.Fd()), syscall.LOCK_UN)
	if shareErr != nil {
		t.Fatalf("a second reader could not share the lock without waiting: %v", shareErr)
	}
	if err != nil || !writerHeldOff {
		t.Fatalf("TryLock while two readers hold the lock = (held %v, err %v), want held: a writer waits for the readers", writerHeldOff, err)
	}
	release, held, err := flock.TryLock(path)
	if err != nil || held {
		t.Fatalf("TryLock after both readers released = (held %v, err %v), want the lock", held, err)
	}
	release()
}

func TestLockShared_NeverCreatesTheLockFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.lock")

	_, err := flock.LockShared(path)

	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("LockShared on a missing lock file = %v, want fs.ErrNotExist", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("LockShared created %s (stat err=%v): a reader leaves nothing behind", path, statErr)
	}
}
