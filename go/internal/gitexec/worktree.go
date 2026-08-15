package gitexec

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// DefaultWorktreeAddAttempts bounds the provisioning retry: first try + two
// retries. Two is enough for lock-window collisions at the standing fleet
// width; a third identical failure is a real condition the fail-fast must
// surface. This is the ONE bound every `git worktree add` call site shares —
// core, swarm and the operator CLI all read it from here rather than keeping
// private copies (the copy-paste the consolidation exists to remove).
const DefaultWorktreeAddAttempts = 3

// WorktreeAddRetry carries the per-caller knobs of the shared retry loop. The
// zero value is usable: real time.Sleep, no retry announcement. Passing the
// knobs as a VALUE (rather than reading exported mutable package globals) keeps
// the loop single-sourced while letting each caller keep its own test seams.
type WorktreeAddRetry struct {
	// Sleep is the inter-attempt backoff clock — a seam so package tests count
	// sleeps instead of paying them. Nil ⇒ time.Sleep.
	Sleep func(time.Duration)

	// OnRetry, when non-nil, is called before each RE-attempt (never before the
	// first) with the 1-based attempt index, the total bound, and the failing
	// attempt's exit code + stderr, so a caller can announce contention in its
	// own voice. Nil ⇒ silent retry.
	OnRetry func(attempt, attempts, code int, stderr string)

	// Retryable, when non-nil, classifies a failed attempt BEFORE any backoff
	// is paid: false ⇒ the condition is permanent, so the loop returns the
	// failure immediately instead of sleeping the ladder for nothing.
	//
	// Why it exists: the loop retried on ANY non-zero exit, so a permanent
	// `fatal: not a git repository` (rc=128) bought the full 2s+4s ladder. In
	// go/cmd/evolve, 33 tests transitively reach this loop over a t.TempDir()
	// that is not a repository — 33 × 6s = 198s of pure sleep in a package the
	// build floor runs with `-timeout 120s`. Deterministic, not a flake.
	//
	// Nil ⇒ today's retry-everything, so the zero value and every existing
	// caller keep their behaviour unchanged. This narrows only WHEN the loop
	// sleeps: a persistent failure still returns the final exit code and git's
	// own stderr intact (see the contract below) — the fail-fast alarm chain
	// stays armed, which is what refuted PR #400 got wrong.
	Retryable func(code int, stderr string) bool
}

// permanentWorktreeAddStderr are the `git worktree add` failure conditions no
// amount of waiting can change. The list is a DENY-list on purpose: contention
// is the open-ended class (rc=255 with nothing on stderr beyond "Preparing
// worktree" is the live incident shape), so anything unrecognised stays
// retryable and PR #401's collision absorber is preserved verbatim. Only
// conditions proven permanent are subtracted from it.
var permanentWorktreeAddStderr = []string{
	"not a git repository",   // rc=128 — the 198s cmd/evolve tax
	"is already checked out", // the branch is live in another worktree
	"already exists",         // the destination path or branch is taken
}

// RetryableWorktreeAddFailure is the shared transience classifier for
// WorktreeAddRetry.Retryable. It lives here, beside the loop, so core, swarm
// and the operator CLI classify identically instead of each re-deriving "which
// failures are worth waiting for" — the same single-sourcing argument that put
// DefaultWorktreeAddAttempts here.
//
// Unrecognised ⇒ retryable. A misclassified transient costs a lane its cycle;
// a misclassified permanent costs 6s. The asymmetry sets the default.
func RetryableWorktreeAddFailure(code int, stderr string) bool {
	low := strings.ToLower(stderr)
	for _, marker := range permanentWorktreeAddStderr {
		if strings.Contains(low, marker) {
			return false
		}
	}
	return true
}

// AddWorktreeWithRetry runs `git worktree add <args...>` with the bounded,
// backoff'd retry proven by PR #401 (a497ffe1).
//
// Why it exists: N lanes of one repo provision concurrently and `git worktree
// add` takes repo-level locks in the SHARED .git; the observed collision returned
// rc=255 with nothing on stderr beyond "Preparing worktree". One transient collision
// used to cost a lane its whole cycle (ActiveWorktree stayed empty, CB.2
// fail-fasted every dispatch, three identical fingerprints halted the batch).
// The fix landed at exactly one call site; this is that loop lifted to the
// package every provisioning site already depends on, so the three siblings
// adopt it instead of re-deriving it.
//
// Contract:
//   - A clean add costs exactly ONE invocation and ZERO backoff — the retry is a
//     collision absorber, not a rate limiter that taxes every cycle.
//   - A PERSISTENT failure still fails loudly after the bound, returning the
//     final exit code and git's own stderr intact: the downstream fail-fast
//     alarm chain is CORRECT and must stay armed (refuted PR #400 is the record
//     of what happens when the alarm is silenced instead).
//   - The `worktree add` prefix is prepended HERE, so no call site can drift
//     into a different subcommand while claiming this retry contract.
func (g Git) AddWorktreeWithRetry(ctx context.Context, r WorktreeAddRetry, args ...string) (stdout, stderr string, exitCode int, err error) {
	sleep := r.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	argv := append([]string{"worktree", "add"}, args...)
	var firstFailure string
	for attempt := 0; attempt < DefaultWorktreeAddAttempts; attempt++ {
		stdout, stderr, exitCode, err = g.Capture(ctx, argv...)
		if err == nil && exitCode == 0 {
			return stdout, stderr, exitCode, nil
		}
		if attempt == DefaultWorktreeAddAttempts-1 {
			break // bound reached — surface the failure, don't sleep past it
		}
		// Classify BEFORE sleeping: backoff bought on a permanent condition is
		// pure latency, and the announcement below would be claiming contention
		// that was never established.
		if r.Retryable != nil && !r.Retryable(exitCode, stderr) {
			break
		}
		if firstFailure == "" {
			firstFailure = fmt.Sprintf("initial worktree add failure (rc=%d): %s", exitCode, stderr)
		}
		if r.OnRetry != nil {
			r.OnRetry(attempt+1, DefaultWorktreeAddAttempts, exitCode, stderr)
		}
		sleep(time.Duration(attempt+1) * 2 * time.Second)
	}
	if firstFailure != "" {
		stderr = firstFailure + "\nfinal worktree add failure: " + stderr
	}
	return stdout, stderr, exitCode, err
}
