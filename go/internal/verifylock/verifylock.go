// Package verifylock serializes the host's expensive go-test verification runs.
package verifylock

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

const lockFileName = "evolve-verify.lock"

const pollInterval = 250 * time.Millisecond

const waitNoteAfter = 5 * time.Second

const renoteEvery = 60 * time.Second

const MaxWait = 15 * time.Minute

func Acquire(ctx context.Context, projectRoot string, warn io.Writer) (func(), error) {
	path, hub := lockPathFor(projectRoot)
	if !hub && warn != nil {
		fmt.Fprintf(warn, "[verify] WARN: hub lock unresolvable for %s — degrading to a PER-WORKTREE lock (%s): concurrent lanes are NOT serialized\n", projectRoot, path)
	}
	return AcquireAt(ctx, path, warn)
}

func AcquireWithin(ctx context.Context, projectRoot string, wait time.Duration, warn io.Writer) (func(), error) {
	waitCtx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	release, err := Acquire(waitCtx, projectRoot, warn)
	if err != nil && ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
		return nil, fmt.Errorf("verifylock: no turn at the host verification lock within the %s wait bound, so this run goes unserialized: %w", wait, err)
	}
	return release, err
}

func lockPathFor(projectRoot string) (path string, hub bool) {
	if info, err := plane.Classify(projectRoot); err == nil {
		if common, cerr := plane.CommonGitDir(info); cerr == nil {
			return filepath.Join(common, lockFileName), true
		}
	}
	return filepath.Join(projectRoot, ".evolve", lockFileName), false
}

func AcquireAt(ctx context.Context, lockPath string, warn io.Writer) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, fmt.Errorf("verifylock: %w", err)
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("verifylock: %w", err)
	}
	start := time.Now()
	nextNote := waitNoteAfter
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			_ = f.Close()
			return nil, fmt.Errorf("verifylock: flock: %w", err)
		}
		if warn != nil && time.Since(start) > nextNote {
			fmt.Fprintf(warn, "[verify] single-flight: waiting %s for the host verification lock (another lane is verifying) — %s\n", time.Since(start).Round(time.Second), lockPath)
			nextNote += renoteEvery
		}
		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, fmt.Errorf("verifylock: %w", ctx.Err())
		case <-time.After(pollInterval):
		}
	}
	var once sync.Once
	release := func() {
		once.Do(func() {
			_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
			_ = f.Close()
		})
	}
	return release, nil
}
