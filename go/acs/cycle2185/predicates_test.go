//go:build acs

package cycle2185

import (
	"path/filepath"
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
			t.Errorf("test %s did NOT pass in %s (missing, failing, or not compiled).\ncombined go-test output:\n%s", name, pkg, acsassert.OutputTail(out, 40))
		}
	}
}

func TestC2185_001_ClaudeTranscriptGivesOrderedTurnLinesWithCountsAndToolNames(t *testing.T) {
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

func TestC2185_002_Agy1967ConversationGives111TurnLinesSummingToTheRow(t *testing.T) {
	assertNamedTestsPass(t, tokenusagePkg,
		"TestTurns_Agy14ConversationGivesOneTurnPerGenerationSummingToTheRow",
		"TestTurns_AgyMalformedGenerationRowIsNotATurn",
	)
	assertNamedTestsPass(t, bridgePkg,
		"TestEngineLaunch_Agy14TurnsFillTheSidecarAndSumToTheRow",
	)
}

func TestC2185_003_CallAboveTheLineCapWritesTheCapAndATruncatedCount(t *testing.T) {
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

func TestC2185_004_TurnSumMismatchWarnsAndLeavesTheRowUnchanged(t *testing.T) {
	assertNamedTestsPass(t, bridgePkg,
		"TestEngineLaunch_TurnSumMismatchWarnsAndLeavesTheRowAlone",
	)
}

func TestC2185_005_TurnWriteFailureWarnsAndNeverFailsTheLaunch(t *testing.T) {
	assertNamedTestsPass(t, llmcallsPkg,
		"TestAppendTurns_NoTurnsCreatesNoFile",
		"TestAppendTurns_UnwritableSidecarReturnsAnErrorNamingThePath",
	)
	assertNamedTestsPass(t, bridgePkg,
		"TestEngineLaunch_TurnWriteFailureWarnsAndNeverFailsTheLaunch",
	)
}

// acs-predicate: config-check
func TestC2185_006_PackageNoteStatesToolNamesComeFromEveryLineOfAMessageId(t *testing.T) {
	note := filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "packages", "internal-tokenusage.md")
	if !acsassert.FileNotContains(t, note, "The last line of a message id wins, so its tool names come from that line.") {
		t.Errorf("the package note still records the removed last-line-wins tool-name rule as the design (audit M1, cycle 2175)")
	}
	if !acsassert.FileMatchesRegex(t, note, `(?i)tool names[^.\n]*(all|every)[^.\n]*line`) {
		t.Errorf("the package note does not say that the tool names of a turn come from all the in-window lines of its message id")
	}
}
