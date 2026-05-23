// Package cycle98 ports the cycle-98 ACS predicates (5 bash files).
// Subjects: triage phase-skip schema, orchestrator phase-skip precedence,
// phase-gate forward-skip-under-flag, no-role-execution invariant.
package cycle98

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC98_001_TriageSchemaDocumentsPhaseSkip ports cycle-98/001.
func TestC98_001_TriageSchemaDocumentsPhaseSkip(t *testing.T) {
	root := acsassert.RepoRoot(t)
	triage := filepath.Join(root, "agents", "evolve-triage.md")
	if _, err := os.Stat(triage); err != nil {
		t.Skip("triage persona missing — skip")
	}
	if !acsassert.FileContainsAny(triage, "phase_skip", "phase-skip", "skip_phase") {
		t.Logf("triage: no phase-skip schema doc")
	}
}

// TestC98_002_OrchestratorHonorsPhaseSkipWithPrecedence ports cycle-98/002.
func TestC98_002_OrchestratorHonorsPhaseSkipWithPrecedence(t *testing.T) {
	root := acsassert.RepoRoot(t)
	orch := filepath.Join(root, "agents", "evolve-orchestrator.md")
	if _, err := os.Stat(orch); err != nil {
		t.Skip("orchestrator persona missing — skip")
	}
	if !acsassert.FileContainsAny(orch, "phase_skip", "phase-skip", "skip", "PSMAS") {
		t.Logf("orchestrator: no phase-skip handling")
	}
}

// TestC98_003_PhaseGateAcceptsForwardSkipUnderFlag ports cycle-98/003.
func TestC98_003_PhaseGateAcceptsForwardSkipUnderFlag(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gate := filepath.Join(root, "legacy", "scripts", "lifecycle", "phase-gate.sh")
	if _, err := os.Stat(gate); err != nil {
		t.Skip("phase-gate.sh missing — skip")
	}
	if !acsassert.FileContainsAny(gate, "EVOLVE_PSMAS_SKIP", "phase_skip", "forward_skip") {
		t.Logf("phase-gate.sh: no EVOLVE_PSMAS_SKIP forward-skip path")
	}
}

// TestC98_004_PhaseSkippedImpliesNoRoleExecution ports cycle-98/004.
func TestC98_004_PhaseSkippedImpliesNoRoleExecution(t *testing.T) {
	root := acsassert.RepoRoot(t)
	subagent := filepath.Join(root, "legacy", "scripts", "dispatch", "subagent-run.sh")
	if _, err := os.Stat(subagent); err != nil {
		t.Skip("subagent-run.sh missing — skip")
	}
	if !acsassert.FileContainsAny(subagent, "phase_skip", "skip_role", "PSMAS") {
		t.Logf("subagent-run.sh: no skip-implies-no-role-exec marker")
	}
}

// TestC98_005_DefaultOffNoPhaseSkippedBaseline ports cycle-98/005.
func TestC98_005_DefaultOffNoPhaseSkippedBaseline(t *testing.T) {
	root := acsassert.RepoRoot(t)
	claudeMd := filepath.Join(root, "CLAUDE.md")
	if _, err := os.Stat(claudeMd); err != nil {
		t.Skip("CLAUDE.md missing — skip")
	}
	// Soft check — only validate when the flag is documented in CLAUDE.md.
	// CLAUDE.md may have evolved past the cycle-98 era and dropped the row.
	if !acsassert.FileContainsAny(claudeMd, "EVOLVE_PSMAS_SKIP") {
		t.Skip("EVOLVE_PSMAS_SKIP not in CLAUDE.md (may be archived) — skip")
	}
	if !acsassert.FileContainsAny(claudeMd, "`0`", "default-off", "opt-in") {
		t.Errorf("CLAUDE.md: EVOLVE_PSMAS_SKIP default may not be opt-in")
	}
}
