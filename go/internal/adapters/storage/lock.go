package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// lockHooks holds injectable seams for syscall.Flock + file close so
// tests can drive the flock EWOULDBLOCK (held-externally) and arbitrary
// flock-error branches without spawning a sibling process.
type lockHooks struct {
	flock  func(fd int, how int) error
	closeF func(f *os.File) error
}

var lhooks = lockHooks{
	flock:  syscall.Flock,
	closeF: func(f *os.File) error { return f.Close() },
}

func withLockHooks(replacement lockHooks, fn func()) {
	prev := lhooks
	if replacement.flock != nil {
		lhooks.flock = replacement.flock
	}
	if replacement.closeF != nil {
		lhooks.closeF = replacement.closeF
	}
	defer func() { lhooks = prev }()
	fn()
}

// AcquireLock takes the project-scoped .evolve/.lock via syscall.Flock
// with LOCK_NB. Within the same process a sync.Mutex enforces the
// exclusive invariant — Flock alone does not deduplicate same-process
// callers on macOS / Linux.
//
// The returned release function flips both gates (mutex + flock) and
// is safe to call exactly once. Calling it twice is a no-op.
func (s *FilesystemStorage) AcquireLock(_ context.Context) (func() error, error) {
	return s.pl.acquire()
}

// processLock combines a process-local "is held" flag with an OS-level
// flock so concurrent goroutines (in this process) and concurrent
// processes (sharing the file) both see exclusive ownership semantics.
//
// The mutex protects access to the held/file state — it is NOT held
// for the lifetime of the lock (that would deadlock a second AcquireLock
// caller that should instead get ErrLockHeld).
type processLock struct {
	path string
	mu   sync.Mutex
	held bool
	file *os.File
}

func newProcessLock(path string) *processLock {
	return &processLock{path: path}
}

func (p *processLock) acquire() (func() error, error) {
	p.mu.Lock()
	if p.held {
		p.mu.Unlock()
		return nil, fmt.Errorf("%w: project lock at %s already held by this process", core.ErrLockHeld, p.path)
	}
	if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
		p.mu.Unlock()
		return nil, fmt.Errorf("mkdir lock dir: %w", err)
	}
	f, err := os.OpenFile(p.path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		p.mu.Unlock()
		return nil, fmt.Errorf("open lock %s: %w", p.path, err)
	}
	if err := lhooks.flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		p.mu.Unlock()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("%w: project lock at %s held externally", core.ErrLockHeld, p.path)
		}
		return nil, fmt.Errorf("flock %s: %w", p.path, err)
	}
	p.held = true
	p.file = f
	p.mu.Unlock()

	var once sync.Once
	var releaseErr error
	return func() error {
		once.Do(func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			if !p.held {
				return
			}
			_ = lhooks.flock(int(p.file.Fd()), syscall.LOCK_UN)
			if cerr := lhooks.closeF(p.file); cerr != nil {
				releaseErr = fmt.Errorf("close lock: %w", cerr)
			}
			p.file = nil
			p.held = false
		})
		return releaseErr
	}, nil
}
