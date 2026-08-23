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
	"os"
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
// Error discipline, case by case:
//   - HEAD itself unreadable → error (nothing sane to record).
//   - HEAD readable but its ANCESTRY is not → the captured HEAD is returned
//     VERBATIM with a WARN. The walk exists only to detect salvage snapshots,
//     and a snapshot can only be identified by reading subjects; when they
//     cannot be read, recording HEAD verbatim is exactly what the pre-guard
//     code did, whereas an error here would leave the base EMPTY at the call
//     site — which the pre-guard code itself documented as the worse outcome
//     (an empty base disables the cycle-156 normalize). Pinned by
//     TestWorktreeReuseBase_UnreadableAncestryRecordsHEADVerbatim and, one
//     level up, by TestVerdictCacheCollisionRegression, whose missing-base
//     scenario is CONSTRUCTED by stubbing `rev-parse HEAD` — capture must
//     therefore go through `rev-parse HEAD`, never be inferred from the walk.
//   - HEAD is a snapshot with no non-snapshot ancestor within the bound →
//     errUnresolvableSnapshotBase (loud abort): both fallbacks reproduce the
//     defect this guard exists to fix.
func resolveWorktreeBaseSHA(ctx context.Context, worktree string) (string, error) {
	head, code, err := gitCapture(ctx, worktree, "rev-parse", "HEAD")
	if err != nil || code != 0 {
		return "", fmt.Errorf("worktree base: rev-parse HEAD in %s: rc=%d: %w", worktree, code, err)
	}
	head = strings.TrimSpace(head)

	// %H<US>%s per line, newest first: one git call classifies the whole walk.
	// The unit separator cannot appear in a commit subject, so the split is
	// unambiguous.
	out, code, err := gitCapture(ctx, worktree, "log",
		fmt.Sprintf("--max-count=%d", maxSnapshotAncestorWalk), "--format=%H%x1f%s", head)
	if err != nil || code != 0 || strings.TrimSpace(out) == "" {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN worktree base: ancestry of %s unreadable in %s (rc=%d, %v) — recording HEAD verbatim; snapshot detection skipped this cycle\n",
			head, worktree, code, err)
		return head, nil
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
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
