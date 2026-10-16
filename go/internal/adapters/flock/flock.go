// Package flock provides the blocking and non-blocking cross-process file locks of the adapters.
// See docs/architecture/packages/internal-adapters-flock.md.
package flock

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

var flockFn = syscall.Flock

func ShipLockPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".evolve", "ship.lock")
}

func Lock(path string) (release func(), err error) {
	f, err := openLockFile(path)
	if err != nil {
		return nil, err
	}
	return lockFile(f, path, syscall.LOCK_EX)
}

func openLockFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("flock mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("flock open %s: %w", path, err)
	}
	return f, nil
}

func LockShared(path string) (release func(), err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("flock open %s: %w", path, err)
	}
	return lockFile(f, path, syscall.LOCK_SH)
}

func lockFile(f *os.File, path string, how int) (release func(), err error) {
	err = flockFn(int(f.Fd()), how)
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
