//go:build acs

package cycle1566

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const shipPkg = "./internal/phases/ship"

func requirePassing(t *testing.T, pkg string, names ...string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	pattern := "^(" + strings.Join(names, "|") + ")$"
	cmd := exec.Command("go", "test", "-count=1", "-v", "-run", pattern, pkg)
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	text := string(out)
	for _, name := range names {
		if !strings.Contains(text, "--- PASS: "+name) {
			t.Errorf("%s %s: no `--- PASS: %s` line (test missing, failing, or package did not build)\nrun error: %v\noutput:\n%s",
				pkg, pattern, name, err, tail(text))
			return
		}
	}
	if err != nil {
		t.Errorf("%s %s: go test exited non-zero (%v)\noutput:\n%s", pkg, pattern, err, tail(text))
	}
}

func tail(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > 60 {
		lines = append([]string{fmt.Sprintf("... (%d earlier lines elided)", len(lines)-60)}, lines[len(lines)-60:]...)
	}
	return strings.Join(lines, "\n")
}

func TestC1566_001_NewlyAddedFailingTestBlocksShip(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_NewlyAddedFailingTestBlocksShip")
}

func TestC1566_002_NewlyAddedSkippedTestDoesNotBlockShip(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_NewlyAddedSkippedTestDoesNotBlockShip")
}

func TestC1566_003_SelectionBoundedToNewlyAddedTestFiles(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_AddedTestSelectionIgnoresModifiedAndNonTestFiles")
}

func TestC1566_004_ProductionCallerStopsBeforeShip(t *testing.T) {
	requirePassing(t, shipPkg, "TestPhaseRunNative_NewlyAddedFailingTestPreventsRun")
}

func TestC1566_005_SkippedReproducerDoesNotBlockProductionPath(t *testing.T) {
	requirePassing(t, shipPkg, "TestRunNative_AddedSkippedTestDoesNotBlockShip")
}

func TestC1566_006_TaggedFailingAddedTestIsNotSilentlyGreen(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_NewlyAddedTaggedFailingTestIsNotSilentlyGreen")
}

func TestC1566_007_TagGuardedGreenPackageIsNotFalseRed(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_NewlyAddedTagGuardedGreenPackageIsNotFalseRed")
}

func TestC1566_008_LiteralBuildConstraintIsNotAnExclusion(t *testing.T) {
	requirePassing(t, shipPkg, "TestBugReproduction_AddedTestLiteralBuildConstraintIsNotExcluded")
}

func TestC1566_009_DiscoveryFailureIsRecorded(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_AddedTestDiscoveryFailureIsRecorded")
}

func TestC1566_010_RedMessagesAreAttributable(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_RedMessagesDistinguishFixedPackFromAddedTests")
}

func TestC1566_011_ExistingGateContractPreserved(t *testing.T) {
	requirePassing(t, shipPkg,
		"TestRepoContractGate_OffSkips",
		"TestRepoContractGate_EnforceGreenPasses",
		"TestRepoContractGate_EnforceRedFailsWithDedicatedCode",
		"TestRepoContractGate_UnknownStageFailsTowardEnforce",
		"TestRepoContractGate_TransientFailureRetriesOnceThenShips",
		"TestRepoContractGate_PersistentAmbiguityIsInfraClassedExactlyTwoRuns",
	)
}
