// Package verifylock serializes the host's EXPENSIVE go-test verification
// runs (ADR-0080 P1). Fleet lanes are separate OS processes that each shell
// full package suites during EGPS and the build handoff floor; running them
// concurrently oversubscribes one host and turns long suites into false reds
// (batch-16: TouchedPackagesStayGreen red on three lanes, green in the
// preserved worktree — the halt's entire fingerprint). One flock, hub-
// resident so every lane of a repo shares it; LLM phases stay parallel —
// only verification is single-flight.
package verifylock

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

// lockFileName inside the git common dir (the hub — shared by every worktree
// of the repo, the same cross-plane location discipline as the console lease).
const lockFileName = "evolve-verify.lock"

// pollInterval between try-locks; flock has no ctx-aware blocking mode, so
// waiters poll — coarse is fine, verification runs are tens of seconds.
const pollInterval = 250 * time.Millisecond

// waitNoteAfter is how long a waiter stays silent before its first queue
// note; renoteEvery re-notes with elapsed time (review MEDIUM: a suite hold
// spans tens of minutes — a one-shot note reads as a hang to operators and
// to the stall observer).
const waitNoteAfter = 5 * time.Second

const renoteEvery = 60 * time.Second

// Acquire takes the host-wide verification lock for projectRoot's repo,
// blocking (ctx-aware) until it is free. Returns an idempotent release. A
// root whose hub cannot be resolved degrades to a lock file beside the
// project root — still correct for a single checkout, but PER-WORKTREE (no
// cross-lane exclusion), so the degradation is LOUD (review MEDIUM: a silent
// fallback is indistinguishable from working single-flight until the false
// reds return). Verification is never SKIPPED because locking failed.
func Acquire(ctx context.Context, projectRoot string, warn io.Writer) (func(), error) {
	path, hub := lockPathFor(projectRoot)
	if !hub && warn != nil {
		fmt.Fprintf(warn, "[verify] WARN: hub lock unresolvable for %s — degrading to a PER-WORKTREE lock (%s): concurrent lanes are NOT serialized\n", projectRoot, path)
	}
	return AcquireAt(ctx, path, warn)
}

// lockPathFor resolves the hub-resident lock path; hub=false marks the
// per-worktree fallback.
func lockPathFor(projectRoot string) (path string, hub bool) {
	if info, err := plane.Classify(projectRoot); err == nil {
		if common, cerr := plane.CommonGitDir(info); cerr == nil {
			return filepath.Join(common, lockFileName), true
		}
	}
	return filepath.Join(projectRoot, ".evolve", lockFileName), false
}

// AcquireAt is Acquire against an explicit lock path (tests; single source of
// the flock discipline).
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
