package flock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
)

var (
	heldMu    sync.Mutex
	heldPaths = map[string]bool{}
	absFn     = filepath.Abs
)

func TryLock(path string) (release func(), held bool, err error) {
	abs, aerr := absFn(path)
	if aerr != nil {
		abs = path
	}
	heldMu.Lock()
	if heldPaths[abs] {
		heldMu.Unlock()
		return nil, true, nil
	}
	heldMu.Unlock()

	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, false, fmt.Errorf("flock mkdir: %w", err)
	}
	f, err := os.OpenFile(abs, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, false, fmt.Errorf("flock open %s: %w", abs, err)
	}
	err = flockFn(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	runtime.KeepAlive(f)
	if err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("flock %s: %w", abs, err)
	}
	heldMu.Lock()
	heldPaths[abs] = true
	heldMu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			_ = flockFn(int(f.Fd()), syscall.LOCK_UN)
			runtime.KeepAlive(f)
			_ = f.Close()
			heldMu.Lock()
			delete(heldPaths, abs)
			heldMu.Unlock()
		})
	}, false, nil
}
