//go:build acs

package cycle1544

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-count=1", "-v", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("binding test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag). exit=%d\ncombined go-test output:\n%s",
				name, pkg, code, out)
		}
	}
}

func TestC1544_001_APlantedBindingNeverClearsTheLostLandingFloor(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestDetectLostLanding_PlantedShipBindingDoesNotClearTheFloor",
		"TestDetectLostLanding_ValidatedBindingClearsTheFloor",
		"TestDetectLostLanding_CorruptBindingIsNotAWitness",
		"TestDetectLostLanding_HostileActiveWorktreeCannotRedirectTheWitness",
	)
}

func TestC1544_002_SuccessfulShipExitsAreNeverFlaggedLandingLost(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestDetectLostLanding_ShippedViaBuildExitIsNotFlagged",
		"TestDetectLostLanding_CleanNoOpExitIsNotFlagged",
		"TestFinalizeCycle_ShipLatchAndProjectRootReachTheFloor",
	)
}

func TestC1544_003_TheWitnessIsCycleOwnedNeverRepoWide(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestDetectLostLanding_SiblingCycleBindingDoesNotCreditThisCycle",
		"TestDetectLostLanding_WitnessNeverConsultsRepoHEAD",
	)
}

func TestC1544_004_LostLandingEvidenceReachesTheCommittedDossier(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestDossierSystemFailure_LostLandingReachesTheCommittedDossier",
		"TestDossierSystemFailure_AbnormalEpilogueThreadsTheSignal",
	)
}

func TestC1544_005_LandedSiblingAndOrdinaryPassCarryNoEvidence(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestDossierSystemFailure_LandedSiblingCarriesNone",
		"TestDossierSystemFailure_OrdinaryPassStaysByteClean",
	)
}

func TestC1544_006_ReusedSnapshotNeverBecomesTheWorktreeBase(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWorktreeReuseBase_SalvageSnapshotHEADResolvesToFirstNonSalvageAncestor",
	)
}

func TestC1544_007_OrdinaryReuseAndUnresolvableAncestorBehaviour(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWorktreeReuseBase_OrdinaryHEADIsRecordedVerbatim",
		"TestWorktreeReuseBase_UnresolvableSnapshotAncestorFailsLoudly",
	)
}

func TestC1544_008_LandedRegressionsNotWeakened(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestDetectLostLanding_RealCycle1535IsNotAPass",
		"TestDetectLostLanding_RealCycle1536Landed",
		"TestDetectLostLanding_CycleThatNeverShippedIsNotFlagged",
		"TestDetectLostLanding_OnlyShippingVerdictsAreFlagged",
		"TestDetectLostLanding_NoWorkspaceDoesNotReadTheProcessCWD",
		"TestDetectLostLanding_DoesNotHaltTheBatch",
		"TestLostLandingVerdict_IsLegalAndNotShipping",
		"TestFinalizeCycle_LostLandingDowngradesTheVerdictAndRecordsTheSignal",
		"TestFinalizeCycle_LandedCycleIsUnchanged",
		"TestWriteCycleDossier_WritesValidArtifact",
		"TestWriteCycleDossier_FailOutcomeRecordsDefect",
		"TestWriteCycleDossier_LeavesCleanTree",
		"TestOrchestrator_ProvisionsWorktree_PassesToSourcePhases",
		"TestOrchestrator_WorktreeProvisionFailure_BestEffort",
	)
}
