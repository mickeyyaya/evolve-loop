// Package treediff is Workstream B's post-phase main-tree diff guard.
//
// The bridge's OS sandbox confines source-writing phases to their worktree at
// runtime, but the trust-kernel design wants belt-and-suspenders. Each git
// worktree is a separate working directory: `git diff --name-only HEAD` run
// in the main repo root only shows changes to the main tree, NOT to any
// worktree. So after a source-writing phase runs, comparing the main-tree
// dirty set before-and-after isolates exactly the leaks the sandbox is
// supposed to prevent — any newly-dirty *main-tree* path = a write that
// escaped the worktree (Issue 2 / cycle-119 cross-CLI trust bypass).
//
// This package is pure logic: it takes a git-exec seam as a function value so
// the orchestrator can inject test fakes. No package globals.
package treediff

import (
	"context"
	"fmt"
)

// GitDirtyFn returns the set of dirty tracked paths in repoRoot (the output of
// `git diff --name-only HEAD`, one path per line). The orchestrator passes its
// production impl; tests pass a fake.
type GitDirtyFn func(ctx context.Context, repoRoot string) ([]string, error)

// Guard captures the pre-phase snapshot + the seam to query after.
type Guard struct {
	gitDirty GitDirtyFn
}

// New constructs a Guard bound to a git-exec seam.
func New(gitDirty GitDirtyFn) *Guard {
	return &Guard{gitDirty: gitDirty}
}

// Snapshot captures the main-tree dirty set BEFORE a source-writing phase
// runs. A snapshot error is non-fatal at the guard level: the orchestrator
// callers degrade to "skip the post-phase check" rather than abort the cycle
// — observability beats hard-failing on a transient git read.
func (g *Guard) Snapshot(ctx context.Context, repoRoot string) ([]string, error) {
	if g == nil || g.gitDirty == nil {
		return nil, nil // no seam → no-op
	}
	return g.gitDirty(ctx, repoRoot)
}

// CheckResult carries the verdict of a post-phase compare. Leaked is the
// sorted list of paths that became dirty during the phase but were not dirty
// before. SnapshotMissed=true means the before-snapshot itself failed
// (caller couldn't establish a baseline); the orchestrator should warn but
// not abort.
type CheckResult struct {
	Leaked         []string
	SnapshotMissed bool
}

// OK reports whether the check found no leaks (and the snapshot worked).
func (r CheckResult) OK() bool { return len(r.Leaked) == 0 && !r.SnapshotMissed }

// Error formats the leak set for the orchestrator's "abort cycle" path.
// Returns nil when there are no leaks. The format names the phase, worktree,
// and every leaked path so the operator can audit the bypass at a glance.
func (r CheckResult) Error(phase, worktree string) error {
	if r.OK() {
		return nil
	}
	if r.SnapshotMissed {
		return fmt.Errorf("tree-diff guard: pre-phase snapshot failed for %s — leak detection skipped (continuing; trust kernel falls back to Claude-only PreToolUse)", phase)
	}
	return fmt.Errorf("tree-diff guard: phase %q wrote to the main tree outside its worktree %q — leaked paths: %v",
		phase, worktree, r.Leaked)
}

// Check is the post-phase comparison. `before` is the snapshot the orchestrator
// captured before the runner ran; `repoRoot` is the same root used at snapshot.
// Returns the leak set + a SnapshotMissed flag the caller uses to decide
// abort-vs-warn. Errors from the git seam degrade to SnapshotMissed (the guard
// is belt-and-suspenders to the sandbox — it must never be the cause of a
// false abort).
func (g *Guard) Check(ctx context.Context, repoRoot string, before []string) CheckResult {
	if g == nil || g.gitDirty == nil {
		return CheckResult{} // OK by default when guard is disabled
	}
	after, err := g.gitDirty(ctx, repoRoot)
	if err != nil {
		return CheckResult{SnapshotMissed: true}
	}
	beforeSet := make(map[string]struct{}, len(before))
	for _, p := range before {
		beforeSet[p] = struct{}{}
	}
	var leaked []string
	for _, p := range after {
		if _, was := beforeSet[p]; was {
			continue
		}
		leaked = append(leaked, p)
	}
	return CheckResult{Leaked: leaked}
}
