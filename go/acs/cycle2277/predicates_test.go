//go:build acs

package cycle2277

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

func requireCoreTestsPass(t *testing.T, names ...string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", "-v", "-run", "^("+strings.Join(names, "|")+")$", corePkg)
	if code < 0 {
		t.Fatalf("go test did not launch: %v", err)
	}
	report := stdout + stderr
	if code != 0 {
		t.Errorf("RED: core tests %v exit %d\n%s", names, code, acsassert.OutputTail(report, 25))
		return
	}
	for _, name := range names {
		if !strings.Contains(report, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass", name)
		}
	}
}

func TestC2277_001_ControlBytesInThePhaseAndReasonsNeverReachTheBuildPrompt(t *testing.T) {
	requireCoreTestsPass(t,
		"TestReadContinuationFindings_StripsControlBytesFromPhaseAndReasons",
		"TestReadContinuationFindings_CleanReasonsRenderUnchanged",
	)
}

func TestC2277_002_ControlBytesInTheRawBodyFallbackNeverReachTheBuildPrompt(t *testing.T) {
	requireCoreTestsPass(t,
		"TestReadContinuationFindings_StripsControlBytesFromTheRawBodyFallback",
		"TestReadContinuationFindings_AReasonlessArtifactIsSanitizedAsRawText",
		"TestReadContinuationFindings_AMultiLineRawBodyKeepsReadableText",
	)
}

func TestC2277_003_AReasonCannotForgeALineInTheFindingsBlock(t *testing.T) {
	requireCoreTestsPass(t, "TestReadContinuationFindings_AReasonCannotForgeAnotherLine")
}

func TestC2277_004_TheTruncationCutNeverSplitsAMultibyteRune(t *testing.T) {
	requireCoreTestsPass(t,
		"TestReadContinuationFindings_NeverSplitsARuneAtTheBudget",
		"TestTruncateFindings_NeverSplitsARuneAndReportsTheDroppedBytes",
		"TestTruncateFindings_CutsAtLineBoundary",
	)
}

func TestC2277_005_ABodyWithNoReadableTextServesNoFindings(t *testing.T) {
	requireCoreTestsPass(t, "TestReadContinuationFindings_ABodyOfOnlyControlBytesServesNothing")
}

func TestC2277_006_TheSanitizedFindingsReachTheBuildRequestAndTheConsoleBrief(t *testing.T) {
	requireCoreTestsPass(t,
		"TestRunCycle_AContinuationBuildContextCarriesNoControlBytesFromTheFailureReasons",
		"TestConsoleFindings_CarryNoControlBytesFromTheFailureReasons",
		"TestRunCycle_AContinuationWithAnInheritedRedLeadsWithTheRepairDirective",
	)
}
