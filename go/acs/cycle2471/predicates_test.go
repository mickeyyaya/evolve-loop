//go:build acs

package cycle2471

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC2471_001_TwoTodosThatEditDifferentFilesOfALeafPackageBothAdmit(t *testing.T) {
	names := []string{
		"TestAdmit_TwoTodosThatEditDifferentFilesOfALeafPackageBothAdmit",
		"TestAdmit_ALaneInFlightOnOneFileOfALeafLeavesAnotherFileFree",
		"TestAdmit_AFileKeyShowsTheFileInTheCells",
		"TestAdmit_FileKeysAddNoReachToALeaf",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./internal/admission",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2471_002_TheSectionFiveGuardTestsHoldTheSecondTodo(t *testing.T) {
	names := []string{
		"TestAdmit_TwoTodosThatEditTheSameFileOfALeafPackageHoldTheSecond",
		"TestAdmit_TwoTodosThatEditTheSameTestFileOfALeafPackageHoldTheSecond",
		"TestAdmit_AWholePackageEditOfALeafHoldsAFileEditOfThatLeaf",
		"TestAdmit_TwoWholePackageEditsOfALeafHoldTheSecond",
		"TestAdmit_TwoTodosThatEditDifferentFilesOfANonLeafPackageHoldTheSecond",
		"TestAdmit_AWholePackageEditOfALeafShowsThePackageInTheCells",
		"TestAdmit_FileKeysKeepTheHubRule",
		"TestAdmit_FileKeysKeepTheFamilyRule",
		"TestAdmit_FileKeysSendNoReachShadowForAFileHold",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./internal/admission",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2471_003_TheAdmissionPackageStaysGreenWithTheNeverConflictProperty(t *testing.T) {
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
func TestC2471_004_ThePackagePageDescribesTheFileKeys(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := "docs/architecture/packages/internal-admission.md"
	page := filepath.Join(root, filepath.FromSlash(rel))
	if !acsassert.FileExists(t, page) {
		t.Fatalf("RED: %s is missing", rel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Fatalf("RED: %s is untracked", rel)
	}
	for _, needle := range []string{"file_cells", "file key", "whole package", "cmd/evolve/a.go"} {
		if !acsassert.FileContains(t, page, needle) {
			t.Errorf("RED: the package page does not describe %q", needle)
		}
	}
}

func TestC2471_005_AnEmptyFileCellsListChangesNothingAndACmdEvolveEntryFreesTheFiles(t *testing.T) {
	names := []string{
		"TestAdmit_WithNoFileCellsTwoFilesOfALeafStillShareOneCell",
		"TestAdmit_TwoTodosThatEditDifferentFilesOfALeafPackageBothAdmit",
		"TestAdmit_TwoAdmittedLeafTodosNeverShareAFileOrAWholePackageOverRandomBacklogs",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./internal/admission",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2471_006_ThePolicyKeyFileCellsParsesAndRefusesBadEntries(t *testing.T) {
	names := []string{
		"TestFleetConfig_TheFileCellsKeyNamesTheLeafPackagesWithFileKeys",
		"TestFleetConfig_AnUnsetFileCellsKeyIsEmptyAndLeavesTheConfigAtItsDefaults",
		"TestParse_RefusesABadFileCellsKey",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./internal/policy",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2471_007_TheCensusAndThePoolAdmissionTakeTheFileCells(t *testing.T) {
	names := []string{
		"TestCensus_TwoItemsThatEditDifferentFilesOfALeafAreBothAdmissibleWhenTheLeafHasFileKeys",
		"TestAdmitByCell_AFileCellLeafAdmitsTheNextFileAtOnce",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./internal/orchestrator",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2471_008_TheLoopCallersReadFileCellsFromThePolicy(t *testing.T) {
	names := []string{
		"TestCellAdmission_PolicyFileCellsLetTwoFilesOfCmdEvolveRunBesideEachOther",
		"TestAdmissionShadow_PolicyFileCellsReachTheWaveShadow",
		"TestSupplyWiring_TheCensusTakesTheFileCellsOfThePolicy",
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./cmd/evolve",
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}
