package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// worktree_clean.go — clean-HEAD assertion for REUSED per-cycle worktrees.
//
// Cycle-653 incident (fix of record = the cycle-584 lesson's prescribed gate,
// never landed until now): a reused worktree carried a prior failed attempt's
// uncommitted orphan RED test; ship binds the whole `git diff HEAD` tree, so
// the inherited dirt fails (or ships) the cycle regardless of any phase's
// notion of scope — cycle 653 would have PASSed in isolation. Family:
// cycle-24, -93, -365, -584, -645, -653.
//
// Policy (single mechanism, applied only on the gitWorktree.Create REUSE
// branch — a fresh `worktree add ... HEAD` is clean by construction, and the
// resume path (RunCycleFromPhase) never calls Create, so preserved mid-cycle
// work is untouched): dirty paths are MOVED to a per-cycle quarantine dir for
// salvage (never deleted), then the worktree is hard-reset to HEAD. Fail-loud:
// any quarantine/reset failure aborts provisioning rather than handing a
// dirty worktree to the cycle.
// minimal: the inbox item's alternative "cut a fresh worktree instead" mode is
// deliberately not implemented — quarantine+reset satisfies every acceptance
// criterion, and a config knob selecting between two equivalent outcomes would
// be flag sprawl (no-feature-flags rule). Upgrade path: a policy.json
// worktree.dirty block if a second behavior is ever genuinely needed.

// quarantineDir is where a cycle's evicted worktree dirt is preserved.
func quarantineDir(projectRoot string, cycle int) string {
	return filepath.Join(projectRoot, ".evolve", "quarantine", fmt.Sprintf("cycle-%d", cycle))
}

// ensureCleanWorktree asserts the reused worktree wt is clean at HEAD. Dirty
// paths (tracked-modified AND untracked, file-granular via porcelain -uall)
// are moved under quarantineDir preserving their relative layout, then the
// worktree is `git reset --hard HEAD`. Returns the quarantined destination
// paths. A clean worktree is a no-op (no quarantine dir created).
func ensureCleanWorktree(ctx context.Context, wt, projectRoot string, cycle int) ([]string, error) {
	dirty := porcelainDirtySet(ctx, wt)
	if len(dirty) == 0 {
		return nil, nil
	}
	paths := make([]string, 0, len(dirty))
	for p := range dirty {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	qdir := quarantineDir(projectRoot, cycle)
	var moved []string
	for _, p := range paths {
		src := filepath.Join(wt, p)
		if _, err := os.Lstat(src); err != nil {
			// Deleted-tracked path or the old side of a rename: nothing on disk
			// to preserve; the reset below restores it from HEAD.
			continue
		}
		dst := uniqueQuarantinePath(filepath.Join(qdir, p))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return moved, fmt.Errorf("worktree quarantine: mkdir for %s: %w", p, err)
		}
		if err := os.Rename(src, dst); err != nil {
			return moved, fmt.Errorf("worktree quarantine: move %s -> %s: %w", src, dst, err)
		}
		moved = append(moved, dst)
		fmt.Fprintf(os.Stderr, "[worktree] WARN dirty reuse (cycle %d): quarantined %s -> %s\n", cycle, p, dst)
	}
	// Restore tracked content (modified/staged/deleted) to HEAD. The dirty
	// content itself is already preserved above, so this destroys nothing.
	if out, code, err := gitCapture(ctx, wt, "reset", "--hard", "HEAD"); err != nil || code != 0 {
		return moved, fmt.Errorf("worktree clean-provision: git reset --hard HEAD in %s (rc=%d): %v: %s", wt, code, err, out)
	}
	fmt.Fprintf(os.Stderr, "[worktree] WARN reused worktree %s was DIRTY: %d path(s) quarantined under %s (preserved for salvage, never deleted); tree reset to HEAD\n", wt, len(moved), qdir)
	return moved, nil
}

// uniqueQuarantinePath suffixes .1, .2, … when a prior attempt at the same
// cycle already quarantined a file of the same name — earlier salvage is never
// overwritten.
func uniqueQuarantinePath(p string) string {
	if _, err := os.Lstat(p); err != nil {
		return p
	}
	for i := 1; ; i++ {
		c := fmt.Sprintf("%s.%d", p, i)
		if _, err := os.Lstat(c); err != nil {
			return c
		}
	}
}
