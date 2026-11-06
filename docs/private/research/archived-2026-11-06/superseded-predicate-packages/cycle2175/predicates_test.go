//go:build acs

package cycle2175

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	tokenusagePkg = "github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
	llmcallsPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	bridgePkg     = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

func assertNamedTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-v", "-count=1", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("test %s did NOT pass in %s (missing, failing, or not compiled).\ncombined go-test output:\n%s", name, pkg, out)
		}
	}
}

func TestC2175_001_ClaudeTranscriptGivesOrderedTurnLinesWithCountsAndToolNames(t *testing.T) {
	assertNamedTestsPass(t, tokenusagePkg,
		"TestTurns_ClaudeTranscriptGivesOneTurnPerMessageInTimeOrder",
		"TestTurns_ClaudeTurnSumEqualsTheRowUsage",
		"TestTurns_NoMatchingTranscriptGivesNoTurns",
		"TestTurns_ParallelToolCallsOfOneMessageKeepEveryToolNameInOrder",
		"TestTurns_ToolNamesSpreadOverTextAndToolLinesKeepOnlyToolUse",
		"TestTurns_RepeatedToolUseLineOfOneMessageIsCountedOnce",
		"TestTurns_ToolNamesOutsideTheWindowAreNotAttributedToTheTurn",
	)
	assertNamedTestsPass(t, llmcallsPkg,
		"TestAppendTurns_OneLinePerTurnInOrderKeyedByCallID",
		"TestTurnsPath_IsTheSidecarInTheWorkspace",
	)
	assertNamedTestsPass(t, bridgePkg,
		"TestEngineLaunch_TurnSidecarHoldsOneLinePerTurnKeyedByTheRowCallID",
		"TestEngineLaunch_ResultWithoutTurnsWritesNoSidecarAndNoTurnSignal",
	)
}

func TestC2175_002_Agy1967ConversationGives111TurnLinesSummingToTheRow(t *testing.T) {
	assertNamedTestsPass(t, tokenusagePkg,
		"TestTurns_Agy14ConversationGivesOneTurnPerGenerationSummingToTheRow",
		"TestTurns_AgyMalformedGenerationRowIsNotATurn",
	)
	assertNamedTestsPass(t, bridgePkg,
		"TestEngineLaunch_Agy14TurnsFillTheSidecarAndSumToTheRow",
	)
}

func TestC2175_003_CallAboveTheLineCapWritesTheCapAndATruncatedCount(t *testing.T) {
	assertNamedTestsPass(t, llmcallsPkg,
		"TestAppendTurns_CapIsWideEnoughForTheLargestKnownCall",
		"TestAppendTurns_CallAboveTheCapWritesTheCapAndATruncatedCount",
		"TestAppendTurns_CallAtTheCapIsNotTruncated",
		"TestAppendTurns_TheCapAppliesToEachCallAlone",
	)
	assertNamedTestsPass(t, bridgePkg,
		"TestEngineLaunch_CallAboveTheLineCapWritesTheCapAndATruncatedCount",
	)
}

func TestC2175_004_TurnSumMismatchWarnsAndLeavesTheRowUnchanged(t *testing.T) {
	assertNamedTestsPass(t, bridgePkg,
		"TestEngineLaunch_TurnSumMismatchWarnsAndLeavesTheRowAlone",
	)
}

func TestC2175_005_TurnWriteFailureWarnsAndNeverFailsTheLaunch(t *testing.T) {
	assertNamedTestsPass(t, llmcallsPkg,
		"TestAppendTurns_NoTurnsCreatesNoFile",
		"TestAppendTurns_UnwritableSidecarReturnsAnErrorNamingThePath",
	)
	assertNamedTestsPass(t, bridgePkg,
		"TestEngineLaunch_TurnWriteFailureWarnsAndNeverFailsTheLaunch",
	)
}
