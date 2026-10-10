package gc

import (
	"strings"
	"testing"
	"time"
)

func TestApplyWorktrees_RemovesTheWorktreeBeforeItsBranch(t *testing.T) {
	e := newWorktreesTestEnv(t)
	const leaf = "cycle-ccc7777-702"
	wt := e.addWorktree(leaf, leaf, 20*time.Hour, false, true)

	opts := e.opts()
	m, err := PlanWorktrees(opts)
	if err != nil {
		t.Fatalf("PlanWorktrees: %v", err)
	}
	if err := ApplyWorktrees(opts, m); err != nil {
		t.Fatalf("ApplyWorktrees: %v", err)
	}

	e.git.mu.Lock()
	defer e.git.mu.Unlock()
	removeAt, branchAt := -1, -1
	for i, c := range e.git.calls {
		if removeAt < 0 && strings.Contains(c, "worktree remove "+wt) {
			removeAt = i
		}
		if branchAt < 0 && strings.Contains(c, "branch -d "+leaf) {
			branchAt = i
		}
	}
	if removeAt < 0 || branchAt < 0 {
		t.Fatalf("want a worktree remove and a branch -d for %s: calls=%v", leaf, e.git.calls)
	}
	if removeAt > branchAt {
		t.Errorf("branch -d ran before the worktree remove, which git refuses for a checked-out branch: calls=%v", e.git.calls)
	}
}
