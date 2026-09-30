//go:build acs

package cycle1562

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	corePkg   = "github.com/mickeyyaya/evolve-loop/go/internal/core"
)

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

func TestC1562_001_WedgedPromptShortCircuitsTheSilenceBudget(t *testing.T) {
	assertDefaultSuiteTestsPass(t, bridgePkg,
		"TestTmuxREPL_PromptSubmitWedged_ShortCircuitsSilenceBudget",
	)
}

func TestC1562_002_CleanAndSilentPanesAreNeverDeliveryFailures(t *testing.T) {
	assertDefaultSuiteTestsPass(t, bridgePkg,
		"TestTmuxREPL_CleanSubmit_NeverClassifiesDeliveryFailure",
		"TestTmuxREPL_SilentPaneTimeout_NotClassifiedAsDeliveryFailure",
	)
}

func TestC1562_003_WedgedNudgeCarriesItsClassifiedCause(t *testing.T) {
	assertDefaultSuiteTestsPass(t, bridgePkg,
		"TestTmuxREPL_NudgeSubmitWedged_ClassifiedCauseSurvivesIntoMarker",
	)
}

func TestC1562_004_RecoveryStaysBoundedAtOneRelaunch(t *testing.T) {
	assertDefaultSuiteTestsPass(t, bridgePkg,
		"TestTmuxREPL_NudgeUnsubmitted_ResendBounded",
		"TestTmuxREPL_NudgeSubmitted_NoResend",
	)
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestOrchestrator_PhaseArtifactTimeout_RetriesAndRecovers",
		"TestOrchestrator_PhaseArtifactTimeout_AbortsAfterCap",
	)
}

func TestC1562_005_ClassifiedCauseSurvivesTheBridgeBoundary(t *testing.T) {
	assertDefaultSuiteTestsPass(t, bridgePkg,
		"TestEngineLaunch_PromptSubmitWedged_PhaseErrorCarriesClassifiedCause",
	)
}

func TestC1562_006_TerminalDiagnosticIsMachineReadable(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWritePhaseFailureDiag_DeliveryFailure_IsMachineReadable",
	)
}

func TestC1562_007_NoFalseDeliveryFailureAttribution(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWritePhaseFailureDiag_GenericSilence_NoDeliveryFailureAttribution",
		"TestWritePhaseFailureDiag_NonTimeoutFailure_NoDeliveryFailureAttribution",
	)
}

func TestC1562_008_ExistingTimeoutEvidenceNotWeakened(t *testing.T) {
	assertDefaultSuiteTestsPass(t, bridgePkg,
		"TestEngineLaunch_ArtifactTimeout_ErrorCarriesWaitAndExtends",
		"TestEngineLaunch_NonTimeoutExit_CauseUnchanged",
		"TestRunTmuxREPL_ArtifactTimeout_SilentPaneIsNotTransient",
	)
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestArtifactTimeoutEndToEnd_DiagnosticNeverMutatesRetryOrFailureLearning",
	)
}
