package flock

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestTryLockAcquiresThenReleases(t *testing.T) {
	release, held, err := TryLock(filepath.Join(t.TempDir(), "own.lock"))
	if err != nil {
		t.Fatalf("TryLock: %v", err)
	}
	if held {
		t.Fatal("a fresh path must not report held")
	}
	if release == nil {
		t.Fatal("a successful TryLock must return a release func")
	}
	release()
}

func TestTryLockSecondSameProcessAcquireReportsHeld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "own.lock")
	release, held, err := TryLock(path)
	if err != nil || held {
		t.Fatalf("first TryLock: held=%v err=%v", held, err)
	}
	defer release()

	// A second acquire of the same path from THIS process must refuse — flock
	// alone does not dedup same-process callers cross-platform, so the held-set
	// must make this deterministic on macOS and Linux alike.
	release2, held2, err2 := TryLock(path)
	if err2 != nil {
		t.Fatalf("second TryLock errored: %v", err2)
	}
	if !held2 {
		t.Fatal("second same-process TryLock must report held=true")
	}
	if release2 != nil {
		t.Fatal("a held TryLock must return a nil release func")
	}
}

func TestTryLockReleaseAllowsReacquire(t *testing.T) {
	path := filepath.Join(t.TempDir(), "own.lock")
	release, _, err := TryLock(path)
	if err != nil {
		t.Fatalf("TryLock: %v", err)
	}
	release()

	release2, held2, err2 := TryLock(path)
	if err2 != nil || held2 {
		t.Fatalf("re-acquire after release: held=%v err=%v", held2, err2)
	}
	release2()
}

func TestTryLockEWOULDBLOCKReportsHeld(t *testing.T) {
	old := flockFn
	t.Cleanup(func() { flockFn = old })
	flockFn = func(_ int, how int) error {
		if how != syscall.LOCK_EX|syscall.LOCK_NB {
			t.Fatalf("TryLock must flock with LOCK_EX|LOCK_NB, got how=%d", how)
		}
		return syscall.EWOULDBLOCK
	}
	release, held, err := TryLock(filepath.Join(t.TempDir(), "own.lock"))
	if err != nil {
		t.Fatalf("EWOULDBLOCK must be held, not an error: %v", err)
	}
	if !held {
		t.Fatal("EWOULDBLOCK from flock must report held=true")
	}
	if release != nil {
		t.Fatal("a held TryLock must return a nil release func")
	}
}

func TestTryLockOtherFlockErrorSurfaces(t *testing.T) {
	old := flockFn
	t.Cleanup(func() { flockFn = old })
	want := errors.New("flock broke")
	flockFn = func(_ int, _ int) error { return want }
	_, held, err := TryLock(filepath.Join(t.TempDir(), "own.lock"))
	if err == nil {
		t.Fatal("a non-EWOULDBLOCK flock error must surface")
	}
	if held {
		t.Fatal("a real flock error is not a held report")
	}
}

func TestTryLockOpenErrorSurfaces(t *testing.T) {
	// A directory path cannot be opened O_RDWR → real error, not held.
	_, held, err := TryLock(t.TempDir())
	if err == nil {
		t.Fatal("TryLock on a directory path must fail opening the lock file")
	}
	if held {
		t.Fatal("an open error is not a held report")
	}
}

func TestTryLockReleaseClearsSameProcessHold(t *testing.T) {
	// Belt-and-suspenders: ensure release removes the held-set entry even when
	// the OS flock is stubbed out (so the in-process gate is what we exercise).
	old := flockFn
	t.Cleanup(func() { flockFn = old })
	flockFn = func(_ int, _ int) error { return nil }

	path := filepath.Join(t.TempDir(), "own.lock")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	release, held, err := TryLock(path)
	if err != nil || held {
		t.Fatalf("first: held=%v err=%v", held, err)
	}
	_, held2, _ := TryLock(path)
	if !held2 {
		t.Fatal("same-process second acquire must be held while first is live")
	}
	release()
	release3, held3, err3 := TryLock(path)
	if err3 != nil || held3 {
		t.Fatalf("after release the same-process hold must clear: held=%v err=%v", held3, err3)
	}
	release3()
}
