//go:build acs

package cycle1773

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	cmdPkg  = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
)

func runNamed(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^("
	for i, n := range names {
		if i > 0 {
			pattern += "|"
		}
		pattern += n
	}
	pattern += ")$"
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.RepoRoot(t) + "/go",
		Package: pkg,
		Pattern: pattern,
		Names:   names,
	})
}

func TestC1773_001_CheckedOutRefKeptAndWalkContinues(t *testing.T) {
	runNamed(t, corePkg,
		"TestPruneSupersededOrphans_CheckedOutRefSkippedNotAborted",
		"TestPruneSupersededOrphans_MultipleConsecutiveCheckedOutRefsAllSkipped",
		"TestPruneSupersededOrphans_CheckedOutMatchIsExactBranchName",
	)
}

func TestC1773_002_ContinuationBoundRefNeverPruned(t *testing.T) {
	runNamed(t, corePkg,
		"TestPruneSupersededOrphans_ContinuationBoundIsKept",
		"TestPruneSupersededOrphans_CheckedOutAndBoundReportsOneReason",
	)
}

func TestC1773_003_DeleteFailureReportedPerRef(t *testing.T) {
	runNamed(t, corePkg,
		"TestPruneSupersededOrphans_DeleteFailureReportedPerRefNotAborted",
	)
}

func TestC1773_004_UndecidableKeepAbortsBeforeAnyDelete(t *testing.T) {
	runNamed(t, corePkg,
		"TestPruneSupersededOrphans_WorktreeListFailureAbortsBeforeAnyDelete",
		"TestPruneSupersededOrphans_CorruptRegistryAbortsBeforeAnyDelete",
		"TestPruneSupersededOrphans_HasOpenPRErrorStillAbortsAfterCheckedOutSkip",
	)
}

func TestC1773_005_PruneCLIReportsKeptReasons(t *testing.T) {
	runNamed(t, cmdPkg,
		"TestBranchesPruneForce_KeepsCheckedOutAndBoundPrunesRest",
		"TestBranchesPruneForce_DeleteFailureReportedPerRef",
		"TestBranchesPruneDryRun_KeptReasonsNeverWouldPrune",
	)
}

func TestC1773_006_ExistingPruneBehaviorUnchanged(t *testing.T) {
	runNamed(t, corePkg,
		"TestPruneSupersededOrphans_SupersededNoPRIsPruned",
		"TestPruneSupersededOrphans_OpenPRFlaggedButKept",
		"TestPruneSupersededOrphans_DistinctBranchUntouched",
		"TestPruneSupersededOrphans_HasOpenPRErrorAborts",
	)
	runNamed(t, cmdPkg,
		"TestBranchesAudit_ReportsSupersededAndLandable",
		"TestBranchesPruneDryRunDefault_KeepsSuperseded",
		"TestBranchesPruneForce_DeletesSupersededKeepsDivergent",
	)
}
