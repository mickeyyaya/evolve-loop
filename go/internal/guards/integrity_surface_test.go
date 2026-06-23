package guards

import "testing"

// TestIsProtectedSurface pins the pipeline integrity control plane: the gates,
// metric SSOT, guards, contract, grading rubrics, and hook wiring that no
// autonomous cycle may modify (the cycle-20 breach edited
// go/acs/regression/flagreaders/readers_test.go to bless its own dodge). The
// match is on a slash-normalized fragment so it holds wherever the file lives
// (cycle worktree, branch root, or main).
func TestIsProtectedSurface(t *testing.T) {
	protected := []string{
		// the exact cycle-20 breach file, inside a cycle worktree
		"/Users/x/.evolve/worktrees/cycle-21/go/acs/regression/flagreaders/readers_test.go",
		"/repo/go/acs/regression/flagprogress/progress_test.go",
		"go/internal/guards/role.go",                     // repo-relative form
		"/repo/go/internal/acssuite/acssuite.go",         // the gate runner
		"/wt/go/internal/flagregistry/registry_table.go", // metric SSOT
		"/wt/go/internal/flagregistry/registry_ceiling_test.go",
		"/wt/knowledge-base/research/flag-campaign-plan.json", // the contract
		"/wt/skills/audit/SKILL.md",                           // grading rubric
		"/wt/skills/adversarial-testing/SKILL.md",             // the adversarial anti-gaming rubric (real dir)
		"/wt/.claude/settings.json",                           // repo hook wiring
		"/Users/runner/.claude/settings.json",                 // GLOBAL hook wiring (C1: must not be "always safe")
		"/wt/.evolve/policy.json",                             // M2: gate-default overrides
		"/wt/Go/ACS/Regression/evil_test.go",                  // M1: case-insensitive FS (Go/ACS == go/acs)
	}
	for _, p := range protected {
		if !IsProtectedSurface(p) {
			t.Errorf("IsProtectedSurface(%q) = false, want true (control plane must be protected)", p)
		}
	}

	allowed := []string{
		"/wt/go/internal/core/orchestrator.go",     // ordinary source
		"/wt/go/acs/cycle21/predicates_test.go",    // cycles legitimately write their OWN predicates
		"/wt/go/internal/flagregistry/registry.go", // non-SSOT registry code
		"/wt/go/internal/flagregistry/lookup.go",   // non-SSOT registry code
		"/wt/skills/loop/SKILL.md",                 // a non-grading skill
		"/wt/README.md",
		"",
	}
	for _, p := range allowed {
		if IsProtectedSurface(p) {
			t.Errorf("IsProtectedSurface(%q) = true, want false (must not over-block legit writes)", p)
		}
	}
}
