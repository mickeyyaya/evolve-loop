package core

// worktree_base.go — a reused salvage snapshot must never become the cycle's
// normalization base.
//
// gitWorktree.Create reuses a valid same-cycle worktree and returns it untouched
// (worktree.go:75-90), after which RunCycle records that worktree's current HEAD
// verbatim as cs.WorktreeBaseSHA (cyclerun.go:496-497). When the reused HEAD is
// an ADR-0076 salvage snapshot — the commit stampContinuationManifest makes to
// preserve a failed attempt's work (continuation_stamp.go:44-45) — the base and
// the preserved work are the SAME commit. normalizeWorktreeToBase then soft-
// resets to the snapshot, the salvaged diff reads as empty, and the very work
// salvage exists to protect normalizes away to nothing.
//
// The fix is deliberately narrow: only a snapshot HEAD is walked back, and only
// to its FIRST non-snapshot ancestor. An unconditional walk-back would re-base
// every ordinary lane, which is a strictly worse defect than the one being
// fixed, so ordinary reuse must capture HEAD verbatim and is pinned by test.

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// salvageSnapshotSubject is the commit subject snapshotPreservedWorktree stamps
// on every salvage snapshot. Single-sourced here and consumed by both the
// producer (continuation_stamp.go) and this guard: a subject that drifted on one
// side would silently disarm the other.
const salvageSnapshotSubject = "salvage snapshot (ADR-0076 continuation-on-fail)"

// maxSnapshotAncestorWalk bounds the ancestry walk. A lane that re-failed
// repeatedly stacks one snapshot per attempt, so the walk must tolerate a short
// chain — but an unbounded walk over a long history would turn a provisioning
// step into a repo-wide scan. Beyond the bound the guard fails loudly rather
// than guessing.
const maxSnapshotAncestorWalk = 64

var errUnresolvableSnapshotBase = errors.New("unresolvable salvage snapshot base")

// resolveWorktreeBaseSHA returns the SHA to record as the cycle's
// WorktreeBaseSHA for worktree: its HEAD, unless HEAD is a salvage snapshot, in
// which case the first ancestor that is not one.
//
// It fails LOUDLY (error, never a silent fallback) when the ancestry cannot be
// resolved: falling back to the snapshot reproduces the defect, and falling back
// to an empty base disables normalization entirely (cyclerun.go:501-505), so
// both "recoveries" are worse than an error the caller can surface.
func resolveWorktreeBaseSHA(ctx context.Context, worktree string) (string, error) {
	// %H<US>%s per line, newest first: one git call answers both "what is HEAD"
	// and "which of these are snapshots". The unit separator cannot appear in a
	// commit subject, so the split is unambiguous.
	out, code, err := gitCapture(ctx, worktree, "log",
		fmt.Sprintf("--max-count=%d", maxSnapshotAncestorWalk), "--format=%H%x1f%s", "HEAD")
	if err != nil || code != 0 {
		return "", fmt.Errorf("worktree base: git log in %s: rc=%d: %w", worktree, code, err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return "", fmt.Errorf("worktree base: no commits reachable from HEAD in %s", worktree)
	}
	for _, line := range lines {
		sha, subject, found := strings.Cut(strings.TrimSpace(line), "\x1f")
		if !found || sha == "" {
			continue
		}
		if strings.TrimSpace(subject) != salvageSnapshotSubject {
			return sha, nil
		}
	}
	return "", fmt.Errorf(
		"%w: HEAD of %s is a salvage snapshot and no non-snapshot ancestor was found within %d commits — refusing to normalize onto preserved work",
		errUnresolvableSnapshotBase,
		worktree, maxSnapshotAncestorWalk)
}
