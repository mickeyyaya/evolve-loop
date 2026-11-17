//go:build acs

package cycle2448

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC2448_001_ABoundaryWhoseMergeBringsAKeyOnlyTheMergedCodeReadsCompletesWithTheRefreshAfterTheRebuild(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./cmd/evolve",
		Pattern: "^(TestBoundaryMergedPolicy_TheSyncMainStepOfEachPlanCompletesWhenTheMergeBringsAKeyOnlyTheMergedCodeReads|TestBoundaryMergedPolicy_EachPlanRefreshesTheCodeIndexAfterSyncMainAndTheWavePlanAfterTheBuild|TestBoundaryMergedPolicy_TheRefreshStepRunsTheRebuiltPlaneBinaryAndKeepsItsExitCode|TestBoundaryMergedPolicy_TheBoundaryRunDispatcherHandlesTheRefreshStepOfItsPlan|TestBoundaryMergedPolicy_TheBoundaryOfAMergedReaderAndItsKeyCompletesWithTheRebuiltBinary|TestBoundaryMergedPolicy_TheStepsThatReadThePolicyStrictlyRunAfterTheBuildInEachPlan|TestBoundaryMergedPolicy_TheBoundaryRunPlanReadsTheMergedPolicyWithTheBinaryTheBuildLeavesOnDisk)$",
		Names: []string{
			"TestBoundaryMergedPolicy_TheSyncMainStepOfEachPlanCompletesWhenTheMergeBringsAKeyOnlyTheMergedCodeReads",
			"TestBoundaryMergedPolicy_EachPlanRefreshesTheCodeIndexAfterSyncMainAndTheWavePlanAfterTheBuild",
			"TestBoundaryMergedPolicy_TheRefreshStepRunsTheRebuiltPlaneBinaryAndKeepsItsExitCode",
			"TestBoundaryMergedPolicy_TheBoundaryRunDispatcherHandlesTheRefreshStepOfItsPlan",
			"TestBoundaryMergedPolicy_TheBoundaryOfAMergedReaderAndItsKeyCompletesWithTheRebuiltBinary",
			"TestBoundaryMergedPolicy_TheStepsThatReadThePolicyStrictlyRunAfterTheBuildInEachPlan",
			"TestBoundaryMergedPolicy_TheBoundaryRunPlanReadsTheMergedPolicyWithTheBinaryTheBuildLeavesOnDisk",
		},
	})
}

func TestC2448_002_AKeyNoReaderOfTheMergedTreeKnowsStillFailsTheBoundaryNamingTheKey(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./cmd/evolve",
		Pattern: "^(TestBoundaryMergedPolicy_AKeyNoReaderOfTheMergedTreeKnowsStopsTheBoundaryAtTheFirstStepOfTheRebuiltBinaryNamingTheKey|TestBoundaryMergedPolicy_TheStrictLoaderOfTheRefreshVerbStillRefusesAnUnknownKeyByName)$",
		Names: []string{
			"TestBoundaryMergedPolicy_AKeyNoReaderOfTheMergedTreeKnowsStopsTheBoundaryAtTheFirstStepOfTheRebuiltBinaryNamingTheKey",
			"TestBoundaryMergedPolicy_TheStrictLoaderOfTheRefreshVerbStillRefusesAnUnknownKeyByName",
		},
	})
}

func TestC2448_003_TheOperationsDocRecordsTheOrderOfTheMergeTheBuildAndTheRefresh(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./cmd/evolve",
		Pattern: "^(TestBoundaryMergedPolicy_TheOperationsDocRecordsTheOrderOfTheMergeTheBuildAndTheRefresh|TestBoundaryMergedPolicy_TheOperationsDocRecordsThatTheStepsThatReadThePolicyRunAfterTheBuildInBothPlans)$",
		Names: []string{
			"TestBoundaryMergedPolicy_TheOperationsDocRecordsTheOrderOfTheMergeTheBuildAndTheRefresh",
			"TestBoundaryMergedPolicy_TheOperationsDocRecordsThatTheStepsThatReadThePolicyRunAfterTheBuildInBothPlans",
		},
	})
}
