//go:build acs

package cycle533

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goTestCore(t *testing.T, pattern string) (string, string, int, error) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available — orchestrator leak/guard integration tests require real git")
	}
	corePkg := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "core")
	return acsassert.SubprocessOutput("go", "test", "-tags", "integration", "-count=1",
		"-run", pattern, corePkg)
}

func TestC533_001_CatalogSourceWriterLeakRecovered(t *testing.T) {
	stdout, stderr, code, err := goTestCore(t, "TestGuardRecoversCatalogWritesSourcePhaseLeak")
	if err != nil || code != 0 {
		t.Errorf("catalog source-writer phase leak must be recovered (leak-recovery gate must be catalog-aware), but the behavioural test failed (code=%d err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}

func TestC533_002_NonSourcePhaseLeakStillAborts(t *testing.T) {
	stdout, stderr, code, err := goTestCore(t, "TestGuardStillAbortsNonSourcePhaseLeak")
	if err != nil || code != 0 {
		t.Errorf("non-source phase leak must still abort (recovery must stay scoped to catalog source-writers), but the behavioural test failed (code=%d err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}

func TestC533_003_BuiltinLeakAndGuardSuiteStaysGreen(t *testing.T) {
	pattern := "TestTDDLeakRecover|TestOrchestrator_AuditLeakRecover|" +
		"TestGuardIgnoresOrchestratorSelfWrite_WorktreePhase|" +
		"TestGuardCatchesDeliverableRenameSmuggle_WorktreePhase|" +
		"TestGuardCatchesInsertedPhaseLeak|TestGuardIgnoresLegitimateWorkspaceWrite|" +
		"TestGuardIgnoresScoutEvalMaterialization|TestIsLegitimateMainTreePath"
	stdout, stderr, code, err := goTestCore(t, pattern)
	if err != nil || code != 0 {
		t.Errorf("the pre-existing leak/guard suite must stay green through the fix (built-in tdd/build + classifier paths), but it failed (code=%d err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}

func TestC533_004_CoreVetClean(t *testing.T) {
	corePkg := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "core")
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", "-tags", "integration", corePkg)
	if err != nil || code != 0 {
		t.Errorf("go vet -tags integration ./internal/core/ reported problems (code=%d err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}
