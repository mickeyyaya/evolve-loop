//go:build acs

package cycle2595

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	loopPkg = "./cmd/evolve"
	corePkg = "./internal/core"
	shipPkg = "./internal/phases/ship"
)

func goTests(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^("
	for i, name := range names {
		if i > 0 {
			pattern += "|"
		}
		pattern += name
	}
	pattern += ")$"
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: pkg,
		Pattern: pattern,
		Names:   names,
	})
}

func TestC2595_001_TheLoopTickOpensTheMainRedStopAndEmitsTheIncidentWhenOriginMainCIIsRed(t *testing.T) {
	goTests(t, loopPkg,
		"TestLoopTick_MainCIRedOpensMainRedStopAndEmitsIncident",
		"TestLoopTick_MainCIRedPersistingEmitsOneIncidentAndFilesOneFixItem")
}

func TestC2595_002_ACIOnlyRedOpensTheStopWithoutAnyLaneReproduction(t *testing.T) {
	goTests(t, corePkg,
		"TestMainRedStop_CIOnlyRedNeedsNoLaneReproduction",
		"TestMainRedStop_CIOnlyRedFilesAFixItemThatNamesTheFailedJobsNotALane",
		"TestMainRedStop_AnEmptyFailedListOpensNoStop")
}

func TestC2595_003_ShipRefusesLandingWithTheTypedCICauseWhileOriginMainCIIsRed(t *testing.T) {
	goTests(t, shipPkg,
		"TestShip_RefusesLandingWhileOriginMainCIRed",
		"TestShip_RefusesLandingWhileOriginMainCIRedWithoutRunningTheKeysOfTheStop",
		"TestShip_RefusesALaneOffTheMainTipWithTheCICauseNotARebase")
}

func TestC2595_004_AGreenMainClosesTheStopAndTheLandingsResume(t *testing.T) {
	goTests(t, loopPkg, "TestLoopTick_MainCIGreenClosesTheStopAndConsumesTheFixItem")
	goTests(t, shipPkg, "TestShip_LandingsResumeWhenAGreenMainClosesTheCIStop")
}

func TestC2595_005_AnUnknownCIStateOrALaneReproducedKeyClosesAndOpensNothing(t *testing.T) {
	goTests(t, loopPkg,
		"TestLoopTick_MainCIUnknownOpensNoStopAndEmitsNoIncident",
		"TestLoopTick_MainCIGreenKeepsTheStopOnALaneReproducedKey",
		"TestMainCIPollTick_OriginMainErrorLogsWarningAndOpensNothing")
}

func TestC2595_006_ThePollRunsAtThePolicyIntervalFromTheProductionWiring(t *testing.T) {
	goTests(t, loopPkg,
		"TestLoopTick_MainCIPollHonoursThePolicyInterval",
		"TestLoopTick_MainCIPollWiredToOriginMainAndGH",
		"TestNewLoopMainCIPoll_LoadsPolicyAndFallsBackWhenMissingOrInvalid")
}

func TestC2595_007_ShadowModeLandsAndRecordsTheRefusalWhileOriginMainCIIsRed(t *testing.T) {
	goTests(t, shipPkg, "TestShip_ShadowLandsWhileOriginMainCIRedAndRecordsTheRefusal")
}

func TestC2595_008_OnlyTheLaneThatCarriesTheCIFixItemLandsAndNoOtherStopShapeIsAdmitted(t *testing.T) {
	goTests(t, shipPkg,
		"TestShip_AdmitsTheLaneThatCarriesTheCIStopFixItemAtEnforce",
		"TestShip_AdmitsTheCIFixLaneThatTheTriageDecisionCommits",
		"TestShip_RefusesAnotherLaneWhileTheCIStopHoldsEvenWhenItHasAScope",
		"TestShip_CarryingTheFixItemOfALaneKeyStopStillNeedsTheKeyProof",
		"TestShip_CarryingTheFixItemOfAMixedStopStillNeedsTheKeyProof")
}
