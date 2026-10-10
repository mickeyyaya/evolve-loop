package gc

import (
	"testing"
	"time"
)

func TestApplyWorktrees_FlagOnlyManifestIsNoOp(t *testing.T) {
	e := newWorktreesTestEnv(t)
	wt := e.addWorktree("cycle-flag0-900", "cycle-flag0-900", 20*time.Hour, true, false)

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
