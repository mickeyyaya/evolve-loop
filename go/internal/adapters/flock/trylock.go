package flock

// trylock.go — the NON-BLOCKING companion to Lock. Where Lock serializes
// read-modify-writers by waiting, TryLock refuses: it lets a caller learn that
// another live process already owns a resource and decline rather than queue.
// This is the primitive behind cross-session ownership leases (a second
// `evolve campaign run` on the same goal-hash refuses instead of clobbering).

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
)

// heldPaths records the absolute lock paths THIS process holds via TryLock so a
// second same-process acquire is deterministically reported as held. flock(2)
// alone does not dedup same-process callers cross-platform (the storage project
// lock adds the same in-process guard for exactly this reason), so the OS lock
// covers cross-process exclusion and this set covers same-process exclusion.
var (
	heldMu    sync.Mutex
	heldPaths = map[string]bool{}
)

// TryLock attempts the exclusive lock on path WITHOUT blocking. It returns
// held=true (with a nil release and nil error) when another live holder — this
// process or another — already owns the lock, so callers can implement
// refuse-or-attach ownership semantics. The OS flock is released automatically
// when the holder dies (even on SIGKILL), so a dead owner's lock is immediately
// re-acquirable. On success, call the returned release exactly once — defer it.
func TryLock(path string) (release func(), held bool, err error) {
	abs, aerr := filepath.Abs(path)
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
	// KeepAlive: f must outlive the raw-fd syscall (see Lock).
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
