//go:build acs

package cycle2463

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cmdPackage      = "./cmd/evolve/"
	loopwavePackage = "./internal/loopwave"
)

func requirePass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-v", "-run", pattern, pkg)
	if code < 0 {
		t.Fatalf("go test did not start: %v", err)
	}
	out := stdout + stderr
	if code != 0 {
		t.Errorf("%s: go test exit %d\n%s", pkg, code, acsassert.OutputTail(out, 40))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("%s: %s did not run and pass", pkg, name)
		}
	}
}

func TestC2463_001_ExcludeNeverAssignsAnExcludedIDToALane(t *testing.T) {
	requirePass(t, cmdPackage, "TestSelectorFlags_ExcludeNeverAssignsAnExcludedIDToALane")
	requirePass(t, loopwavePackage,
		"TestPlanSelector_ExcludeKeepsTheItemOffEveryLane",
		"TestPlanSelector_ExcludeAlsoPrunesAPriorTriageDecisionAndTheWiden",
		"TestPlanSelector_ExcludeAlsoKeepsTheItemOutOfTheBacklogRefill")
}

func TestC2463_002_OnlyAssignsOnlyTheListedIDs(t *testing.T) {
	requirePass(t, cmdPackage,
		"TestSelectorFlags_OnlyAssignsOnlyTheListedIDs",
		"TestSelectorFlags_OnlyAndExcludeTogetherNarrowTheListedIDs")
	requirePass(t, loopwavePackage,
		"TestPlanSelector_OnlyAssignsNothingButTheListedItems",
		"TestPlanSelector_OnlyAlsoFiltersAPriorTriageDecisionAndTheWiden",
		"TestPlanSelector_OnlyAlsoLimitsTheBacklogRefill",
		"TestPlanSelector_TheZeroSelectorPlansLikeNoSelector")
}

func TestC2463_003_AnOnlyIDNoLaneCanTakeExits10AtLaunch(t *testing.T) {
	requirePass(t, cmdPackage,
		"TestSelectorFlags_AnOnlyIDNoLaneCanTakeExits10AtLaunch",
		"TestSelectorFlags_TheOnlyCheckRunsWhateverTheGoalText",
		"TestSelectorFlags_TheOnlyCheckRunsAtTheFrontDoor",
		"TestSelectorFlags_OnlyIDsTheLanesCanTakeLaunch",
		"TestSelectorFlags_ARelaunchedLaunchIsNotCheckedAgainstItemsItAlreadyShipped")
}

func TestC2463_004_AnIDInBothListsExits10NamingTheID(t *testing.T) {
	requirePass(t, cmdPackage, "TestSelectorFlags_AnIDInBothListsExits10NamingTheID")
}

func TestC2463_005_NoInboxFileIsChangedByEitherFlag(t *testing.T) {
	requirePass(t, cmdPackage, "TestSelectorFlags_NoInboxFileChanges")
	requirePass(t, loopwavePackage, "TestPlanSelector_PlanningChangesNoInboxFile")
}

func TestC2463_006_TheDetachedChildCarriesTheSelector(t *testing.T) {
	requirePass(t, cmdPackage, "TestSelectorFlags_TheDetachedChildCarriesTheSelector")
}

func TestC2463_007_TheChainReExecCarriesTheSelector(t *testing.T) {
	requirePass(t, cmdPackage, "TestSelectorFlags_TheChainReExecCarriesTheSelector")
}

func TestC2463_008_TheBoundaryLoopStepCarriesTheSelector(t *testing.T) {
	requirePass(t, cmdPackage,
		"TestSelectorFlags_TheBoundaryLoopStepCarriesTheSelector",
		"TestSelectorFlags_TheWaveNextLoopStepCarriesTheSelector")
}
