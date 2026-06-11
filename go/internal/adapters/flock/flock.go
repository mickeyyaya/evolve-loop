// Package flock provides the tiny blocking cross-process file lock shared
// by the adapters that serialize short read-modify-write critical sections
// (ledger.Append CA.1, storage.UpdateState CA.3). BLOCKING by design —
// unlike the storage project lock (LOCK_NB → ErrLockHeld), these writers
// must serialize, never refuse; every critical section is one read + one
// write, so waits are bounded in practice.
//
// flock(2) locks are per open-file-description, so two opens in the SAME
// process also contend — Lock serializes goroutines and processes alike.
package flock

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

// flockFn is the syscall seam so tests can drive flock-error branches
// without a second process.
var flockFn = syscall.Flock

// Lock blocks until the exclusive lock on path is held. The returned
// release unlocks and closes; call it exactly once (idempotence is the
// caller's concern — defer it).
func Lock(path string) (release func(), err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("flock mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("flock open %s: %w", path, err)
	}
	err = flockFn(int(f.Fd()), syscall.LOCK_EX)
	// KeepAlive: f must outlive the raw-fd syscall — without it the GC may
	// finalize (close) f between Fd() and the flock completing.
	runtime.KeepAlive(f)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("flock %s: %w", path, err)
	}
	return func() {
		_ = flockFn(int(f.Fd()), syscall.LOCK_UN)
		runtime.KeepAlive(f)
		_ = f.Close()
	}, nil
}
