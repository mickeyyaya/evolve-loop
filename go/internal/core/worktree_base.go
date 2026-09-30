package core

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

// maxSnapshotAncestorWalk bounds the ancestry walk so a lane that re-failed
// repeatedly can stack snapshots without turning provisioning into a
// repo-wide scan; beyond the bound the guard fails loudly rather than guessing.
const maxSnapshotAncestorWalk = 64

var errUnresolvableSnapshotBase = errors.New("unresolvable salvage snapshot base")

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
