package policy

import "testing"

func TestWorktreeBase_DefaultAndOverride(t *testing.T) {
	if got := (Policy{}).WorktreeBase(); got != "" {
		t.Errorf("absent worktree block: WorktreeBase() = %q, want \"\"", got)
	}

	p := Policy{Worktree: &WorktreePolicy{Base: "/mnt/fast/wt"}}
	if got := p.WorktreeBase(); got != "/mnt/fast/wt" {
		t.Errorf("WorktreeBase() = %q, want /mnt/fast/wt", got)
	}
}
