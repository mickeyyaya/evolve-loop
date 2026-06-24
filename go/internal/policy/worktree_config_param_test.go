package policy

import "testing"

// TestWorktreeBase_DefaultAndOverride locks the policy.json "worktree" block
// that replaces the EVOLVE_WORKTREE_BASE env read (flag-reduction, ADR-0064).
// Absent block ⇒ "" (caller applies its built-in <root>/.evolve/worktrees
// default); a present block flows the operator override through.
func TestWorktreeBase_DefaultAndOverride(t *testing.T) {
	if got := (Policy{}).WorktreeBase(); got != "" {
		t.Errorf("absent worktree block: WorktreeBase() = %q, want \"\"", got)
	}

	p := Policy{Worktree: &WorktreePolicy{Base: "/mnt/fast/wt"}}
	if got := p.WorktreeBase(); got != "/mnt/fast/wt" {
		t.Errorf("WorktreeBase() = %q, want /mnt/fast/wt", got)
	}
}
