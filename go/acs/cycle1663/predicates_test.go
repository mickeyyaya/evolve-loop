//go:build acs

package cycle1663

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg      = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	dossierPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	cycle1544Pkg = "github.com/mickeyyaya/evolve-loop/go/acs/cycle1544"
)

func assertSuiteTestsPass(t *testing.T, tags, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	var (
		stdout, stderr string
		code           int
		err            error
	)
	if tags == "" {
		stdout, stderr, code, err = acsassert.SubprocessOutput("go", "test", "-run", pattern, "-count=1", "-v", pkg)
	} else {
		stdout, stderr, code, err = acsassert.SubprocessOutput("go", "test", "-tags", tags, "-run", pattern, "-count=1", "-v", pkg)
	}
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

func TestC1663_001_LostLandingEvidenceReachesTheCommittedDossier(t *testing.T) {
	assertSuiteTestsPass(t, "", corePkg,
		"TestDossierSystemFailure_LostLandingReachesTheCommittedDossier",
		"TestDossierSystemFailure_AbnormalEpilogueThreadsTheSignal",
	)
}

func TestC1663_002_LandedSiblingAndOrdinaryPassCarryNoEvidence(t *testing.T) {
	assertSuiteTestsPass(t, "", corePkg,
		"TestDossierSystemFailure_LandedSiblingCarriesNone",
		"TestDossierSystemFailure_OrdinaryPassStaysByteClean",
	)
}

func TestC1663_003_SchemaFloorAndLandedRegressionsNotWeakened(t *testing.T) {
	assertSuiteTestsPass(t, "", dossierPkg,
		"TestSchema_NoDrift",
	)
	assertSuiteTestsPass(t, "", corePkg,
		"TestDetectLostLanding_RealCycle1535IsNotAPass",
		"TestDetectLostLanding_RealCycle1536Landed",
		"TestDetectLostLanding_CycleThatNeverShippedIsNotFlagged",
		"TestFinalizeCycle_LostLandingDowngradesTheVerdictAndRecordsTheSignal",
		"TestFinalizeCycle_LandedCycleIsUnchanged",
		"TestWriteCycleDossier_ParamsStructPreservesFixedInputBytes",
		"TestWriteCycleDossier_ParamsAreKeyedAndOptional",
		"TestWriteCycleDossier_WritesValidArtifact",
		"TestWriteCycleDossier_FailOutcomeRecordsDefect",
		"TestWriteCycleDossier_LeavesCleanTree",
		"TestDossierFailure_FailCarriesIdentity",
		"TestDossierFailure_PassKeepsShape",
		"TestAbnormalEpilogue_WritesDossierDigestAndCoherentState",
		"TestAbnormalEpilogue_NoopAfterNormalCloseout",
	)
}

func TestC1663_004_Cycle1544PredicatesRestoredAndGreen(t *testing.T) {
	assertSuiteTestsPass(t, "acs", cycle1544Pkg,
		"TestC1544_004_LostLandingEvidenceReachesTheCommittedDossier",
		"TestC1544_005_LandedSiblingAndOrdinaryPassCarryNoEvidence",
	)
}
