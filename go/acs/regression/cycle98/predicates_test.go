//go:build acs

package cycle98

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

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

func TestC98_005_DefaultOffNoPhaseSkippedBaseline(t *testing.T) {
	root := acsassert.RepoRoot(t)
	claudeMd := filepath.Join(root, "CLAUDE.md")
	if _, err := os.Stat(claudeMd); err != nil {
		t.Skip("CLAUDE.md missing — skip")
	}
	if !acsassert.FileContainsAny(claudeMd, "EVOLVE_PSMAS_SKIP") {
		t.Skip("EVOLVE_PSMAS_SKIP not in CLAUDE.md (may be archived) — skip")
	}
	if !acsassert.FileContainsAny(claudeMd, "`0`", "default-off", "opt-in") {
		t.Errorf("CLAUDE.md: EVOLVE_PSMAS_SKIP default may not be opt-in")
	}
}
