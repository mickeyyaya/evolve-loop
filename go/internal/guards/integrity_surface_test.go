package guards

import "testing"

func TestIsProtectedSurface(t *testing.T) {
	protected := []string{
		"/Users/x/.evolve/worktrees/cycle-21/go/acs/regression/flagreaders/readers_test.go",
		"/repo/go/acs/regression/flagprogress/progress_test.go",
		"go/internal/guards/role.go", // repo-relative form
		"/repo/go/internal/acssuite/acssuite.go",
		"/wt/go/internal/flagregistry/registry_table.go",
		"/wt/go/internal/flagregistry/registry_ceiling_test.go",
		"/wt/knowledge-base/research/flag-campaign-plan.json",
		"/wt/skills/audit/SKILL.md",
		"/wt/skills/adversarial-testing/SKILL.md",
		"/wt/.claude/settings.json",
		"/Users/runner/.claude/settings.json", // global hook wiring, under an always-safe dir
		"/wt/.evolve/policy.json",
		"/wt/Go/ACS/Regression/evil_test.go", // case-insensitive filesystems: Go/ACS is go/acs
		"/wt/go/internal/core/orchestrator.go",
		"/wt/go/internal/core/cyclerun.go",
		"/wt/go/internal/core/cyclerun_dispatch.go",
		"/wt/go/internal/core/cyclerun_review.go",
		"/wt/go/internal/core/resume.go",
		"/wt/go/internal/phases/runner/runner.go",
		"/wt/go/internal/phases/audit/audit.go",
		"/wt/go/internal/phases/retro/retro.go",
		"/wt/go/internal/phases/ship/native.go",
	}
	for _, p := range protected {
		if !IsProtectedSurface(p) {
			t.Errorf("IsProtectedSurface(%q) = false, want true (control plane must be protected)", p)
		}
	}

	allowed := []string{
		"/wt/go/internal/core/observer.go",
		"/wt/go/acs/cycle21/predicates_test.go", // a cycle writes its own predicates
		"/wt/go/internal/flagregistry/registry.go",
		"/wt/go/internal/flagregistry/lookup.go",
		"/wt/skills/loop/SKILL.md", // a skill that grades nothing
		"/wt/README.md",
		"",
	}
	for _, p := range allowed {
		if IsProtectedSurface(p) {
			t.Errorf("IsProtectedSurface(%q) = true, want false (must not over-block legit writes)", p)
		}
	}
}
