//go:build acs

package cycle2587

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runNamed(t *testing.T, pkg, pattern string, names ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{Dir: acsassert.GoModuleDir(t), Package: pkg, Pattern: pattern, Names: names})
}

func TestC2587_001_LoopTickOpensTheStopAndEmitsTheIncidentOnRedMainCI(t *testing.T) {
	runNamed(t, "./cmd/evolve",
		"^(TestLoopTick_MainCIRedOpensMainRedStopAndEmitsIncident|TestLoopTick_MainCIRedPersistingEmitsOneIncidentAndFilesOneFixItem|TestLoopTick_MainCIPollHonoursThePolicyInterval)$",
		"TestLoopTick_MainCIRedOpensMainRedStopAndEmitsIncident",
		"TestLoopTick_MainCIRedPersistingEmitsOneIncidentAndFilesOneFixItem",
		"TestLoopTick_MainCIPollHonoursThePolicyInterval")
}

func TestC2587_002_LoopTickLeavesNoStopWhenCIIsUnknown(t *testing.T) {
	runNamed(t, "./cmd/evolve", "^TestLoopTick_MainCIUnknownOpensNoStopAndEmitsNoIncident$",
		"TestLoopTick_MainCIUnknownOpensNoStopAndEmitsNoIncident")
}

func TestC2587_003_ProductionPollReachesOriginMainThroughGitAndGH(t *testing.T) {
	runNamed(t, "./cmd/evolve", "^TestLoopTick_MainCIPollWiredToOriginMainAndGH$",
		"TestLoopTick_MainCIPollWiredToOriginMainAndGH")
}

func TestC2587_004_PolicyCarriesThePollIntervalWithADefault(t *testing.T) {
	runNamed(t, "./internal/policy", "^TestPolicy_MainCIPollMinutesDefaultsAndOverrides$",
		"TestPolicy_MainCIPollMinutesDefaultsAndOverrides")
}

func TestC2587_005_CIOnlyRedOpensTheStopWithoutALaneReproduction(t *testing.T) {
	runNamed(t, "./internal/core",
		"^(TestMainRedStop_CIOnlyRedNeedsNoLaneReproduction|TestMainRedStop_CIOnlyRedFilesAFixItemThatNamesTheFailedJobsNotALane|TestMainRedStop_CIOnlyRedOnAHeldStopOpensNothingAndFilesNoSecondItem)$",
		"TestMainRedStop_CIOnlyRedNeedsNoLaneReproduction",
		"TestMainRedStop_CIOnlyRedFilesAFixItemThatNamesTheFailedJobsNotALane",
		"TestMainRedStop_CIOnlyRedOnAHeldStopOpensNothingAndFilesNoSecondItem")
}

func TestC2587_006_ARedVerdictThatNamesNoJobOpensNoStop(t *testing.T) {
	runNamed(t, "./internal/core", "^(TestMainRedStop_AnEmptyFailedListOpensNoStop|TestMainCIRedCode_NamesTheGateRepoIncident)$",
		"TestMainRedStop_AnEmptyFailedListOpensNoStop", "TestMainCIRedCode_NamesTheGateRepoIncident")
}

func TestC2587_007_ShipRefusesLandingWhileOriginMainCIRedWithATypedCause(t *testing.T) {
	runNamed(t, "./internal/phases/ship",
		"^(TestShip_RefusesLandingWhileOriginMainCIRed|TestShip_RefusesLandingWhileOriginMainCIRedWithoutRunningTheKeysOfTheStop|TestShip_RefusesALaneOffTheMainTipWithTheCICauseNotARebase)$",
		"TestShip_RefusesLandingWhileOriginMainCIRed",
		"TestShip_RefusesLandingWhileOriginMainCIRedWithoutRunningTheKeysOfTheStop",
		"TestShip_RefusesALaneOffTheMainTipWithTheCICauseNotARebase")
}

func TestC2587_008_ShadowStageLandsAndRecordsTheCIRefusal(t *testing.T) {
	runNamed(t, "./internal/phases/ship", "^TestShip_ShadowLandsWhileOriginMainCIRedAndRecordsTheRefusal$",
		"TestShip_ShadowLandsWhileOriginMainCIRedAndRecordsTheRefusal")
}

func TestC2587_009_TheCIRefusalHasItsOwnShipErrorCode(t *testing.T) {
	runNamed(t, "./internal/shiperr", "^TestCodeMainCIRed_Vocab$", "TestCodeMainCIRed_Vocab")
}

func TestC2587_010_AGreenMainClosesTheStopAndKeepsLaneReproducedKeys(t *testing.T) {
	runNamed(t, "./cmd/evolve",
		"^(TestLoopTick_MainCIGreenClosesTheStopAndConsumesTheFixItem|TestLoopTick_MainCIGreenKeepsTheStopOnALaneReproducedKey)$",
		"TestLoopTick_MainCIGreenClosesTheStopAndConsumesTheFixItem",
		"TestLoopTick_MainCIGreenKeepsTheStopOnALaneReproducedKey")
}

func TestC2587_011_LandingsResumeAfterTheStopCloses(t *testing.T) {
	runNamed(t, "./internal/phases/ship", "^TestShip_LandingsResumeWhenAGreenMainClosesTheCIStop$",
		"TestShip_LandingsResumeWhenAGreenMainClosesTheCIStop")
}

func TestC2587_012_TheLaneThatCarriesTheCIStopFixItemLandsAtEnforce(t *testing.T) {
	runNamed(t, "./internal/phases/ship",
		"^(TestShip_AdmitsTheLaneThatCarriesTheCIStopFixItemAtEnforce|TestShip_AdmitsTheCIFixLaneThatTheTriageDecisionCommits|TestShip_KeepsTheCIFixItemPendingWhileTheCIStopHoldsAfterTheFixLaneLands|TestShip_ShadowRecordsNoRefusalForTheCIFixLane|TestShipStop_TheCIFixLaneIsAdmittedByTheStopCheckWithNoKeyRun)$",
		"TestShip_AdmitsTheLaneThatCarriesTheCIStopFixItemAtEnforce",
		"TestShip_AdmitsTheCIFixLaneThatTheTriageDecisionCommits",
		"TestShip_KeepsTheCIFixItemPendingWhileTheCIStopHoldsAfterTheFixLaneLands",
		"TestShip_ShadowRecordsNoRefusalForTheCIFixLane",
		"TestShipStop_TheCIFixLaneIsAdmittedByTheStopCheckWithNoKeyRun")
}

func TestC2587_013_NoOtherLaneAndNoLaneKeyStopIsAdmittedByCarryingAFixItem(t *testing.T) {
	runNamed(t, "./internal/phases/ship",
		"^(TestShip_RefusesAnotherLaneWhileTheCIStopHoldsEvenWhenItHasAScope|TestShip_CarryingTheFixItemOfALaneKeyStopStillNeedsTheKeyProof|TestShip_CarryingTheFixItemOfAMixedStopStillNeedsTheKeyProof)$",
		"TestShip_RefusesAnotherLaneWhileTheCIStopHoldsEvenWhenItHasAScope",
		"TestShip_CarryingTheFixItemOfALaneKeyStopStillNeedsTheKeyProof",
		"TestShip_CarryingTheFixItemOfAMixedStopStillNeedsTheKeyProof")
}
