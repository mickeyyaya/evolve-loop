package ship

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/paralleltest"
)

const (
	seamRepoContract  = "swaps the repoContractTestFn package variable through swapRepoContractTest"
	seamGoTestJSON    = "swaps the goTestJSONFn package variable through swapGoTestJSON"
	seamMarshalJSON   = "swaps the marshalJSON package variable through stubMarshalJSON"
	seamExecutable    = "swaps the osExecutable package variable through stubRunningExecutable"
	seamMarshalIndent = "swaps the marshalIndentJSON package variable"
	seamStdoutPipe    = "swaps the stdoutPipe package variable"
	seamInboxItem     = "swaps the readInboxItem and marshalInboxItem package variables"
)

var serialTests = map[string]string{
	"ship.TestPhase_TheRepoContractPackRunsOutsideTheShipWindow":                  seamRepoContract,
	"ship.TestAppendQuarantineManifestCoverfix_AnEncodingFailureLeavesNoManifest": seamMarshalIndent,
	"ship.TestRepairSelfSHAPinCoverfix_AnUnlocatableExecutableDeclines":           seamExecutable,
	"ship.TestRepinPostCycleCoverfix_AnUnknownRunningBinaryLeavesThePinUntouched": seamExecutable,
	"ship.TestAppendShipJournal_ReportsEveryWriteFailure":                         seamMarshalJSON,
	"ship.TestDiscardBinaryChurn_NeverRemovesRunningExecutable":                   seamExecutable,
	"ship.TestExplanationViewSHA_HashesTheSealedViewDeterministically":            seamMarshalJSON,
	"ship.TestItemConsumerStage_EveryFailureIsLoudAndLeavesTheItemPickable":       seamInboxItem,
	"ship.TestManualShipSuccess_LeavesUntrackedGoBinEvolvePresent":                seamExecutable,
	"ship.TestRepoContractGate_DiscoveryFailureIsInfra":                           seamRepoContract,
	"ship.TestRepoContractGate_EnforceGreenPasses":                                seamRepoContract,
	"ship.TestRepoContractGate_EnforceRedFailsWithDedicatedCode":                  seamRepoContract,
	"ship.TestRepoContractGate_OffSkips":                                          seamRepoContract,
	"ship.TestRepoContractGate_PersistentAmbiguityIsInfraClassedExactlyTwoRuns":   seamRepoContract,
	"ship.TestRepoContractGate_RealTestFailureIsContractRedWithoutRetry":          seamRepoContract,
	"ship.TestRepoContractGate_RedErrorMessageNamesFailingTests":                  seamRepoContract,
	"ship.TestRepoContractGate_RedMessagesDistinguishFixedPackFromAddedTests":     seamRepoContract,
	"ship.TestRepoContractGate_ScanLogPersistedOnGreenAndRedRuns":                 seamRepoContract,
	"ship.TestRepoContractGate_TheLoopLogCarriesOneSummaryLinePerRun":             seamRepoContract,
	"ship.TestRepoContractGate_WithoutAScanLogTheNotesStayOnStderr":               seamRepoContract,
	"ship.TestRepoContractGate_APassAfterAnInfraRetryIsNotAPlainGreen":            seamRepoContract,
	"ship.TestRepoContractGate_WithoutAScanLogAnInfraRetryStaysOnStderr":          seamRepoContract,
	"ship.TestRepoContractGate_ASecondRunKeepsTheScanLogOfTheFirst":               seamRepoContract,
	"ship.TestRepoContractGate_AnUnopenableScanLogKeepsTheNotesOnStderr":          seamRepoContract,
	"ship.TestRepoContractGate_TransientFailureRetriesOnceThenShips":              seamRepoContract,
	"ship.TestRepoContractGate_UnknownStageFailsTowardEnforce":                    seamRepoContract,
	"ship.TestRepoContractPack_ShipAndTheBuildFloorMakeOneDecision":               seamRepoContract,
	"ship.TestRunGoTestJSON_AStdoutPipeFailureNeverStartsTheProcess":              seamStdoutPipe,
	"ship.TestRunNative_RepoContractGateReceivesRunWorkspace":                     seamRepoContract,
	"ship.TestRunRepoContractPack_NamesRedsOnlyForTheGatesRealRed":                seamRepoContract,
	"ship.TestRunRepoContractPack_RunsTheGatesOwnPackAndNamesItsReds":             seamRepoContract,
	"ship.TestRunRepoContractTestsAlone_EveryPackageGreenIsGreen":                 seamGoTestJSON,
	"ship.TestRunRepoContractTestsAlone_KeepsTheUnprovenAndTheStillRed":           seamGoTestJSON,
}

func TestEveryTopLevelTestIsParallelOrListedSerial(t *testing.T) {
	t.Parallel()
	sources, err := paralleltest.Sources(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range paralleltest.Violations(sources, paralleltest.Config{Serial: serialTests}) {
		t.Error(v)
	}
}

func TestSerialByDefaultGuard_CatchesARealSeamTestMadeParallel(t *testing.T) {
	t.Parallel()
	sources, err := paralleltest.Sources(".")
	if err != nil {
		t.Fatal(err)
	}
	const name = "TestRepoContractGate_OffSkips"
	marker := "func " + name + "(t *testing.T) {\n"
	for path, src := range sources {
		if strings.Contains(src, marker) {
			sources[path] = strings.Replace(src, marker, marker+"\tt.Parallel()\n", 1)
		}
	}
	joined := strings.Join(paralleltest.Violations(sources, paralleltest.Config{Serial: serialTests}), "\n")
	if !strings.Contains(joined, "ship."+name+": calls t.Parallel but is listed as serial") {
		t.Fatalf("a seam-swapping test given t.Parallel must be flagged as listed serial; got %s", joined)
	}
	unlisted := strings.Join(paralleltest.Violations(sources, paralleltest.Config{}), "\n")
	if !strings.Contains(unlisted, "via helper swapRepoContractTest") {
		t.Fatalf("without its serial entry the test must be flagged through swapRepoContractTest; got %s", unlisted)
	}
}
