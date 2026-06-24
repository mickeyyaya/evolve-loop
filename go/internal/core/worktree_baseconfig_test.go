package core

import (
	"path/filepath"
	"testing"
)

// TestGitWorktree_BaseFromConfigNotEnv locks the flag-reduction change (ADR-0064):
// the per-cycle worktree base comes from the injected override (policy.json
// worktree.base, threaded via core.WithWorktreeBase) — NOT the EVOLVE_WORKTREE_BASE
// env var, which is removed. A set env var must be ignored; the struct field is
// the single source.
func TestGitWorktree_BaseFromConfigNotEnv(t *testing.T) {
	t.Setenv("EVOLVE_WORKTREE_BASE", "/env/must/be/ignored")

	g := gitWorktree{baseOverride: "/cfg/base"}
	if got := g.base("/proj"); got != "/cfg/base" {
		t.Errorf("base() with override = %q, want /cfg/base (env must be ignored)", got)
	}

	// No override ⇒ built-in default, env still ignored.
	wantDefault := filepath.Join("/proj", ".evolve", "worktrees")
	if got := (gitWorktree{}).base("/proj"); got != wantDefault {
		t.Errorf("default base() = %q, want %q", got, wantDefault)
	}
}

// TestWithWorktreeBase_SetsProvisionerBase verifies the composition-root option
// threads the resolved policy.json worktree.base into the production provisioner,
// and that an empty base is a no-op (the default provisioner stands).
func TestWithWorktreeBase_SetsProvisionerBase(t *testing.T) {
	o := &Orchestrator{}
	WithWorktreeBase("/mnt/wt")(o)
	gw, ok := o.worktree.(gitWorktree)
	if !ok || gw.baseOverride != "/mnt/wt" {
		t.Fatalf("WithWorktreeBase did not set baseOverride: worktree=%+v ok=%v", o.worktree, ok)
	}

	o2 := &Orchestrator{worktree: gitWorktree{}}
	WithWorktreeBase("")(o2)
	if gw2 := o2.worktree.(gitWorktree); gw2.baseOverride != "" {
		t.Errorf("empty base must not set an override, got %q", gw2.baseOverride)
	}
}
