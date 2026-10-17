//go:build acs

package cycle1925

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const prCmdPkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

func runPRMergerTests(t *testing.T, names ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: prCmdPkg,
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC1925_001_MergeOneWaitsForTheNewHeadAfterUpdateBranch(t *testing.T) {
	runPRMergerTests(t, "TestPRMerger_WaitsForNewHeadAfterUpdateBranch")
}

func TestC1925_002_TrainMergesABehindSecondPRInOneRun(t *testing.T) {
	runPRMergerTests(t, "TestPRMerger_TrainWithUpdateBranch")
}

func TestC1925_003_LaggingHeadIsRefusedInsideTheWaitAndNeverAwaitsCIOnTheOldHead(t *testing.T) {
	runPRMergerTests(t, "TestPRMerger_UpdateBranchTimeout")
}

func TestC1925_004_ExistingPRMergeContractsStayGreen(t *testing.T) {
	runPRMergerTests(t,
		"TestParsePRMergeArgs_InterleavesAndRejectsUsage",
		"TestRunPR_UsageExitsTen",
		"TestPRMerger_WaitsForGreenThenMergesAtTheVerifiedHead",
		"TestPRMerger_RefusesAndStops",
		"TestPRMerger_GHFailuresExitTwo",
	)
}
