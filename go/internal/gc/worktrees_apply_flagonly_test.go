package gc

// worktrees_apply_flagonly_test.go — builder-added coverage (cycle 570) for a
// safety property the RED suite pins only in PlanWorktrees, never in
// ApplyWorktrees: flag-dirty / flag-unmerged items are report-only, so applying
// a manifest that contains ONLY flags must perform ZERO git mutations (no
// worktree remove, no branch delete, no prune) and no trailing prune. Also the
// one place the WorktreeManifest type is constructed by name.

import (
	"testing"
	"time"
)

func TestApplyWorktrees_FlagOnlyManifestIsNoOp(t *testing.T) {
	e := newWorktreesTestEnv(t)
	wt := e.addWorktree("cycle-flag0-900", "cycle-flag0-900", 20*time.Hour, true /*dirty*/, false /*merged*/)

	m := WorktreeManifest{Items: []WorktreeItem{
		{Path: wt, Branch: "cycle-flag0-900", Action: WorktreeActionFlagDirty},
		{Branch: "cycle-orphan-901", Action: WorktreeActionFlagUnmerged},
	}}

	if err := ApplyWorktrees(e.opts(), m); err != nil {
		t.Fatalf("a flag-only manifest must apply cleanly (report-only): %v", err)
	}
	for _, sub := range []string{"worktree remove", "branch -d", "worktree prune"} {
		if n := e.git.callCount(sub); n != 0 {
			t.Errorf("flag-only manifest must never run %q: saw %d (calls=%v)", sub, n, e.git.calls)
		}
	}
}
