//go:build acs

package cycle1555

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

func TestC1555_001_AddedFailingTestBlocksShip(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_NewlyAddedFailingTestBlocksShip")
}

func TestC1555_002_SelectionBoundedToNewlyAddedTestFiles(t *testing.T) {
	requirePassing(t, shipPkg, "TestRepoContractGate_AddedTestSelectionIgnoresModifiedAndNonTestFiles")
}

func TestC1555_003_ExistingGateContractPreserved(t *testing.T) {
	requirePassing(t, shipPkg,
		"TestRepoContractGate_OffSkips",
		"TestRepoContractGate_EnforceGreenPasses",
		"TestRepoContractGate_EnforceRedFailsWithDedicatedCode",
		"TestRepoContractGate_UnknownStageFailsTowardEnforce",
	)
}

func TestC1555_004_ProductionCallerStopsBeforeShip(t *testing.T) {
	requirePassing(t, shipPkg, "TestPhaseRunNative_NewlyAddedFailingTestPreventsRun")
}

func TestC1555_005_SkippedReproducerDoesNotBlock(t *testing.T) {
	requirePassing(t, shipPkg, "TestRunNative_AddedSkippedTestDoesNotBlockShip")
}
