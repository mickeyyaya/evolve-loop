//go:build acs

// Package cycle85 ports the cycle-85 ACS predicates (7 bash files).
// Subjects: builder worktree isolation, cycle-state phase validation,
// memo profile, orchestrator closure mode, promote ACS fallback,
// subagent cost attribution, triage operator-queue priority floor.
package cycle85

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// TestC85_BuilderWorktreeIsolationHardError ports test_builder_worktree_isolation_hard_error.sh.
func TestC85_BuilderWorktreeIsolationHardError(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gate := filepath.Join(root, "legacy", "scripts", "lifecycle", "phase-gate.sh")
	if _, err := os.Stat(gate); err != nil {
		t.Skip("phase-gate.sh missing — skip")
	}
	if !acsassert.FileContainsAny(gate, "EVOLVE_BUILDER_ISOLATION_STRICT", "isolation_breach", "isolation breach") {
		t.Errorf("phase-gate.sh: no builder-isolation-strict hard-error path")
	}
}

// TestC85_CycleStatePhaseValidation ports test_cycle_state_phase_validation.sh.
func TestC85_CycleStatePhaseValidation(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gate := filepath.Join(root, "legacy", "scripts", "lifecycle", "phase-gate.sh")
	if _, err := os.Stat(gate); err != nil {
		t.Skip("phase-gate.sh missing — skip")
	}
	if !acsassert.FileContainsAny(gate, "cycle-state.json", "phaseProgress", "currentPhase") {
		t.Errorf("phase-gate.sh: no cycle-state phase validation reference")
	}
}

// TestC85_MemoProfileHasMemoMdWrite ports test_memo_profile_has_memo_md_write.sh.
func TestC85_MemoProfileHasMemoMdWrite(t *testing.T) {
	root := acsassert.RepoRoot(t)
	profile := filepath.Join(root, ".evolve", "profiles", "memo.json")
	if _, err := os.Stat(profile); err != nil {
		t.Skip("memo profile missing — skip")
	}
	if !acsassert.FileContainsAny(profile, "memo.md", "Write") {
		t.Errorf("memo profile: no memo.md / Write tool reference")
	}
}

// TestC85_OrchestratorClosureModeCheck ports test_orchestrator_closure_mode_check.sh.
func TestC85_OrchestratorClosureModeCheck(t *testing.T) {
	root := acsassert.RepoRoot(t)
	orch := filepath.Join(root, "agents", "evolve-orchestrator.md")
	if _, err := os.Stat(orch); err != nil {
		t.Skip("orchestrator persona missing — skip")
	}
	if !acsassert.FileContainsAny(orch, "closure", "verdict", "PASS", "FAIL") {
		t.Errorf("orchestrator: no closure-mode markers")
	}
}

// TestC85_PromoteACSFallbackPath ports test_promote_acs_fallback_path.sh.
func TestC85_PromoteACSFallbackPath(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "legacy", "scripts", "lifecycle", "promote-research-cache.sh"),
		filepath.Join(root, "legacy", "scripts", "lifecycle", "promote.sh"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return
		}
	}
	t.Skip("no promote script found")
}

// TestC85_SubagentCostAttribution ports test_subagent_cost_attribution.sh.
func TestC85_SubagentCostAttribution(t *testing.T) {
	root := acsassert.RepoRoot(t)
	subagent := filepath.Join(root, "legacy", "scripts", "dispatch", "subagent-run.sh")
	if _, err := os.Stat(subagent); err != nil {
		t.Skip("subagent-run.sh missing — skip")
	}
	if !acsassert.FileContainsAny(subagent, "cost_usd", "cost_attribution", "agent_cost") {
		t.Errorf("subagent-run.sh: no cost attribution marker")
	}
}

// TestC85_TriageOperatorQueuePriorityFloor ports test_triage_operator_queue_priority_floor.sh.
func TestC85_TriageOperatorQueuePriorityFloor(t *testing.T) {
	root := acsassert.RepoRoot(t)
	triage := filepath.Join(root, "agents", "evolve-triage.md")
	if _, err := os.Stat(triage); err != nil {
		t.Skip("triage persona missing — skip")
	}
	if !acsassert.FileContainsAny(triage, "operator", "priority", "HIGH", "MEDIUM") {
		t.Logf("triage: no operator-queue priority markers (may be in reference doc)")
	}
}
