//go:build acs

package cycle2461

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC2461_001_TwoTodosThatEditDifferentFilesOfALeafPackageBothAdmit(t *testing.T) {
	names := []string{"TestAdmit_TwoTodosThatEditDifferentFilesOfALeafPackageBothAdmit",
		"TestAdmit_ALaneInFlightOnOneFileOfALeafLeavesAnotherFileFree",
		"TestAdmit_AFileKeyShowsTheFileInTheCells",
		"TestAdmit_FileKeysAddNoReachToALeaf"}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./internal/admission",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2461_002_TheSectionFiveGuardTestsHoldTheSecondTodo(t *testing.T) {
	names := []string{"TestAdmit_TwoTodosThatEditTheSameFileOfALeafPackageHoldTheSecond",
		"TestAdmit_TwoTodosThatEditTheSameTestFileOfALeafPackageHoldTheSecond",
		"TestAdmit_AWholePackageEditOfALeafHoldsAFileEditOfThatLeaf",
		"TestAdmit_TwoWholePackageEditsOfALeafHoldTheSecond",
		"TestAdmit_TwoTodosThatEditDifferentFilesOfANonLeafPackageHoldTheSecond",
		"TestAdmit_AWholePackageEditOfALeafShowsThePackageInTheCells",
		"TestAdmit_FileKeysKeepTheHubRule",
		"TestAdmit_FileKeysKeepTheFamilyRule",
		"TestAdmit_FileKeysSendNoReachShadowForAFileHold"}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./internal/admission",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2461_003_TheAdmissionPackageStaysGreenWithTheNeverConflictProperty(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./internal/admission",
		Pattern: ".",
		Names: []string{
			"TestAdmit_TwoAdmittedTodosNeverConflict",
			"TestAdmit_TwoAdmittedLeafTodosNeverShareAFileOrAWholePackageOverRandomBacklogs",
			"TestAdmit_OptimisticAdmitsExactlyTheTodosThatShareNoPackageAndAreNotBothHubs",
			"TestAdmit_TheShadowChangesNoDecisionOverRandomBacklogs",
		},
	})
}

// acs-predicate: config-check
func TestC2461_004_ThePackagePageDescribesTheFileKeys(t *testing.T) {
	root := acsassert.RepoRoot(t)
	page := filepath.Join(root, "docs", "architecture", "packages", "internal-admission.md")
	if !acsassert.FileExists(t, page) {
		t.Fatalf("RED: %s is missing", page)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", "docs/architecture/packages/internal-admission.md"); code != 0 {
		t.Fatalf("RED: the package page is untracked")
	}
	for _, needle := range []string{"file_cells", "file key", "whole package", "cmd/evolve/a.go"} {
		if !acsassert.FileContains(t, page, needle) {
			t.Errorf("RED: the package page does not describe %q", needle)
		}
	}
}

func TestC2461_005_AnEmptyFileCellsListChangesNothingAndACmdEvolveEntryFreesTheFiles(t *testing.T) {
	names := []string{"TestAdmit_WithNoFileCellsTwoFilesOfALeafStillShareOneCell",
		"TestAdmit_TwoTodosThatEditDifferentFilesOfALeafPackageBothAdmit",
		"TestAdmit_TwoAdmittedLeafTodosNeverShareAFileOrAWholePackageOverRandomBacklogs"}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./internal/admission",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2461_006_ThePolicyKeyFileCellsParsesAndRefusesBadEntries(t *testing.T) {
	names := []string{"TestFleetConfig_TheFileCellsKeyNamesTheLeafPackagesWithFileKeys",
		"TestFleetConfig_AnUnsetFileCellsKeyIsEmptyAndLeavesTheConfigAtItsDefaults",
		"TestParse_RefusesABadFileCellsKey"}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./internal/policy",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2461_007_TheCensusAndThePoolAdmissionTakeTheFileCells(t *testing.T) {
	names := []string{"TestCensus_TwoItemsThatEditDifferentFilesOfALeafAreBothAdmissibleWhenTheLeafHasFileKeys",
		"TestAdmitByCell_AFileCellLeafAdmitsTheNextFileAtOnce"}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./internal/orchestrator",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2461_008_TheLoopCallersReadFileCellsFromThePolicy(t *testing.T) {
	names := []string{"TestCellAdmission_PolicyFileCellsLetTwoFilesOfCmdEvolveRunBesideEachOther",
		"TestAdmissionShadow_PolicyFileCellsReachTheWaveShadow",
		"TestSupplyWiring_TheCensusTakesTheFileCellsOfThePolicy"}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./cmd/evolve",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}
