package preflight

import (
	"strings"
	"testing"
)

// TestSelectWorktreeBase_WritableOverrideWins locks the flag-reduction change
// (ADR-0064): the operator worktree-base override now flows from Options.WorktreeBase
// (resolved from policy.json worktree.base at the composition root) — NOT the
// EVOLVE_WORKTREE_BASE env var, which is removed. A writable override wins and is
// reported as such.
func TestSelectWorktreeBase_WritableOverrideWins(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	override := t.TempDir() // a real, writable dir

	p := Probe(Options{
		ProjectRoot:  root,
		OSType:       "darwin",
		WorktreeBase: override,
		Env:          stubEnv(map[string]string{"HOME": root}),
		LookPath:     stubLookPath(nil),
		Now:          fixedNow(),
		IsNested:     func() bool { return false },
	})
	if p.AutoConfig.WorktreeBase != override {
		t.Errorf("writable override must win: base = %q, want %q", p.AutoConfig.WorktreeBase, override)
	}
	if !strings.Contains(p.AutoConfig.WorktreeBaseReason, "worktree.base") {
		t.Errorf("reason should attribute the override to worktree.base, got %q", p.AutoConfig.WorktreeBaseReason)
	}
}
