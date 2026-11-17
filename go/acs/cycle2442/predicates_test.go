//go:build acs

package cycle2442

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type namedRun struct {
	pkg   string
	tests []string
}

func passes(t *testing.T, runs ...namedRun) {
	t.Helper()
	goDir := acsassert.GoModuleDir(t)
	for _, run := range runs {
		pattern := "^(" + strings.Join(run.tests, "|") + ")$"
		stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-v", "-run", pattern, run.pkg)
		if code != 0 {
			t.Fatalf("go test -run %s %s: code=%d err=%v\n%s", pattern, run.pkg, code, err, acsassert.OutputTail(stdout+stderr, 40))
		}
		for _, name := range run.tests {
			if !strings.Contains(stdout, "--- PASS: "+name+" ") {
				t.Errorf("%s did not run and pass in %s", name, run.pkg)
			}
		}
	}
}

var (
	trainJourneyOrdering = namedRun{"./internal/integration", []string{
		"TestTrainL2_TheBatchTestRunsBeforeTheBatchPROpensAndTheTestedTipLands",
		"TestTrainL2_AShadowStageRunsNoSeamAndLandsAsBefore",
	}}
	entryPointWiring = namedRun{"./cmd/evolve", []string{
		"TestIntegrationTrainWiresTheBatchTestAtTheStageOfThePolicy",
		"TestJourneyBatchAtShadowRunsNoBatchTestCommand",
	}}
	entryPointHostKill = namedRun{"./cmd/evolve", []string{
		"TestJourneyBatchHostKillOfTheBatchTestEjectsNoMemberAndCreatesNoBatchPR",
	}}
	entryPointJourney = namedRun{"./cmd/evolve", []string{
		"TestJourneyBatchRunsTheBatchTestBeforeTheBatchPRAndBriefsTheMemberThatFailsOnlyOnTheCombinedTree",
	}}
)

func TestC2442_001_TheTrainRunsTheBatchTestBeforeItOpensTheBatchPRThroughTheRealEntryPoint(t *testing.T) {
	passes(t, trainJourneyOrdering, entryPointWiring, entryPointJourney)
}

func TestC2442_002_TwoPRsThatPassAloneAndFailTogetherEjectTheLaterOneWithTheCauseInItsBrief(t *testing.T) {
	passes(t,
		namedRun{"./internal/integration", []string{
			"TestTrainL2_TwoPRsThatPassAloneAndFailTogetherEjectTheLaterOneWithTheCauseInItsBrief",
			"TestBatchTest_ABuildFailureOfThePredicatePackageStillEjects",
		}},
		entryPointJourney,
	)
}

func TestC2442_003_AHostKillEjectsNoMemberAndAGoTestBuildFailureStaysARed(t *testing.T) {
	passes(t,
		namedRun{"./internal/integration", []string{
			"TestTrainL2_AHostKillOfAPackageTestEjectsNoMemberAndOpensNoBatchPR",
			"TestBatchTest_AHostKillOfThePredicateRunEjectsNobodyEvenWithItsSyntheticRed",
			"TestBatchTest_APackStepThatCannotRunIsAHostFailureAndEjectsNobody",
		}},
		namedRun{"./internal/acsrunner", []string{
			"TestHostFailure_WrapsTheCauseAndIsFoundThroughAJoin",
			"TestRun_AGoTestKilledBySignalIsAHostFailure",
			"TestRun_ACancelledContextIsAHostFailure",
			"TestRun_AGoTestThatCannotStartIsAHostFailure",
			"TestRun_ABuildFailureIsNotAHostFailureAndKeepsItsSyntheticRed",
			"TestRun_APredicateRedUnderAKillIsStillAHostFailure",
			"TestRun_ARealPackageThatDoesNotCompileIsABuildFailureAndARealCancelIsAHostFailure",
		}},
		entryPointHostKill,
	)
}

func TestC2442_004_TheL2PackRunsRepoContractCoversBuildVetAndGofmtBeforeL3AndCatchesAGofmtOnlyRed(t *testing.T) {
	passes(t,
		namedRun{"./internal/integration", []string{
			"TestL2Pack_NamesTheSixChecksOfTheL2RowInOrder",
			"TestTrainL2_ATreeThatFailsOnlyGofmtOnTheCombinedTreeEjectsTheMemberBeforeTheBatchPR",
			"TestTrainL2_AFormattedMemberPassesTheWholePackAndLands",
			"TestL2Pack_AFailedStepIsRedByItsNameAndKeepsItsOutput",
			"TestBatchTest_AnEmptyPackKeepsTheLayerAsItWas",
		}},
		entryPointWiring,
		entryPointJourney,
	)
}
