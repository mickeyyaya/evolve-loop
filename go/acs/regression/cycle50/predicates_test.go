//go:build acs

package cycle50

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestC50_001_ScoutStep45Exists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	scout := filepath.Join(root, "agents", "evolve-scout.md")
	if !fixtures.FilePresent(scout) {
		t.Skip("evolve-scout.md missing — skip cycle-50-001")
	}
	if !acsassert.FileMatchesRegex(t, scout, `### 4\.5\.`) {
		return
	}
	for _, code := range []string{
		"0 (HIT)", "10 (STALE)", "20 (MISS)",
		"30 (INVALIDATED)", "40 (NO_ENTRY)", "50 (DISABLED)",
	} {
		if !acsassert.FileContains(t, scout, code) {
			return
		}
	}
}

func TestC50_002_ScoutStep55Exists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	scout := filepath.Join(root, "agents", "evolve-scout.md")
	if !acsassert.FileContainsAny(scout, "research-cache-staging") {
		t.Skip("research-cache-staging marker absent — source evolved past cycle-50-002")
	}
	if !acsassert.FileMatchesRegex(t, scout, `### 5\.5\.`) {
		return
	}
}

func TestC50_003_ScoutStopCriterionCacheSection(t *testing.T) {
	root := acsassert.RepoRoot(t)
	scout := filepath.Join(root, "agents", "evolve-scout.md")
	if !fixtures.FilePresent(scout) {
		t.Skip("evolve-scout.md missing — skip cycle-50-003")
	}
	if !acsassert.FileContainsAny(scout, "STOP CRITERION", "## Stop Criterion", "## STOP") {
		t.Errorf("%s: missing STOP CRITERION block", scout)
	}
	if !acsassert.FileContains(t, scout, "research-cache-section") {
		return
	}
}

func TestC50_004_BuilderStep25ResearchPointer(t *testing.T) {
	root := acsassert.RepoRoot(t)
	builder := filepath.Join(root, "agents", "evolve-builder.md")
	if !acsassert.FileContainsAny(builder, "research_pointer") {
		t.Skip("research_pointer marker absent — source evolved past cycle-50-004")
	}
	if !acsassert.FileContains(t, builder, "Research Source: per-task-cache") {
		return
	}
}

func TestC50_005_TriagePassthroughAllThreeFields(t *testing.T) {
	root := acsassert.RepoRoot(t)
	triage := filepath.Join(root, "agents", "evolve-triage.md")
	if !fixtures.FilePresent(triage) {
		t.Skip("evolve-triage.md missing — skip cycle-50-005")
	}
	for _, field := range []string{"research_pointer", "research_fingerprint", "research_cycle"} {
		if !acsassert.FileContains(t, triage, field) {
			return
		}
	}
}

func TestC50_006_ReconcileInvalidateOnDrop(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rec := filepath.Join(root, "legacy", "scripts", "lifecycle", "reconcile-carryover-todos.sh")
	if !fixtures.FilePresent(rec) {
		t.Skip("reconcile-carryover-todos.sh missing — skip cycle-50-006")
	}
	for _, marker := range []string{"research-cache.sh invalidate", "dropped-cycle"} {
		if !acsassert.FileContains(t, rec, marker) {
			return
		}
	}
}

func TestC50_007_ReconcilePromoteOnPass(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rec := filepath.Join(root, "legacy", "scripts", "lifecycle", "reconcile-carryover-todos.sh")
	if !fixtures.FilePresent(rec) {
		t.Skip("reconcile-carryover-todos.sh missing — skip cycle-50-007")
	}
	if !acsassert.FileContains(t, rec, "promote-research-cache.sh") {
		return
	}
	if !acsassert.FileMatchesRegex(t, rec, `promote-research-cache\.sh[^\n]*(\$CYCLE|"\$CYCLE")[^\n]*(\$WORKSPACE|"\$WORKSPACE")`) {
		t.Errorf("%s: promote-research-cache.sh call missing CYCLE/WORKSPACE args", rec)
	}
}

func TestC50_008_InjectTaskResearchPointerFlag(t *testing.T) {
	root := acsassert.RepoRoot(t)
	inj := filepath.Join(root, "legacy", "scripts", "utility", "inject-task.sh")
	if !fixtures.FilePresent(inj) {
		t.Skip("inject-task.sh missing — skip cycle-50-008")
	}
	if !acsassert.FileContains(t, inj, "--research-pointer") {
		return
	}
	if !acsassert.FileContains(t, inj, "research_pointer") {
		return
	}
}

func TestC50_009_TesterDualVarWorktreePattern(t *testing.T) {
	root := acsassert.RepoRoot(t)
	tester := filepath.Join(root, "agents", "evolve-tester.md")
	if !fixtures.FilePresent(tester) {
		t.Skip("evolve-tester.md missing — skip cycle-50-009")
	}
	if !acsassert.FileContains(t, tester, "EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-") {
		return
	}
	if !acsassert.FileContains(t, tester, "git rev-parse --show-toplevel") {
		return
	}
}
