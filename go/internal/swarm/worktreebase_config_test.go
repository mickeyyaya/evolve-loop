package swarm

import (
	"path/filepath"
	"testing"
)

// TestWorktreeBase_OverrideAndDefault locks the flag-reduction change (ADR-0064):
// the swarm worktree base comes from the injected override (policy.json
// worktree.base, threaded via NewGitWorkerProvisioner) — NOT the EVOLVE_WORKTREE_BASE
// env var, which is removed. An absolute override wins; a relative override is
// refused; an empty override falls back to <root>/.evolve/worktrees.
func TestWorktreeBase_OverrideAndDefault(t *testing.T) {
	if got, err := worktreeBase("/mnt/wt", "/proj"); err != nil || got != "/mnt/wt" {
		t.Fatalf("worktreeBase(override) = %q, %v; want /mnt/wt, nil", got, err)
	}
	if _, err := worktreeBase("relative-base", "/proj"); err == nil {
		t.Fatal("relative override must be refused with an absolute-path error")
	}
	want := filepath.Join("/proj", ".evolve", "worktrees")
	if got, err := worktreeBase("", "/proj"); err != nil || got != want {
		t.Fatalf("worktreeBase(default) = %q, %v; want %q, nil", got, err, want)
	}
}
