//go:build acs

package cycle2495

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const agentRunPackage = "./internal/cli/agentcmd"

func runAgentRunTests(t *testing.T, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	out, _, code, err := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "test", "-count=1", "-run", pattern, "-v", agentRunPackage)
	if err != nil || code != 0 {
		t.Fatalf("go test %s -run %s exit=%d err=%v:\n%s", agentRunPackage, pattern, code, err, acsassert.OutputTail(out, 40))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("%s did not run and pass:\n%s", name, acsassert.OutputTail(out, 40))
		}
	}
}

func TestC2495_001_AReportWhoseOnlyVerdictLineIsInACodeFenceReadsAsTheVerdictItStates(t *testing.T) {
	runAgentRunTests(t,
		"TestAgentRun_AVerdictLineInACodeFenceReadsAsTheVerdictThatItStates",
		"TestReportVerdict_AFencedVerdictStatesTheVerdictThatItReads",
	)
}

func TestC2495_002_TheRegressionTestGoesThroughTheAgentRunEntryPointAndKeepsABlockingFindingBlocking(t *testing.T) {
	runAgentRunTests(t,
		"TestAgentRun_AVerdictLineInACodeFenceReadsAsTheVerdictThatItStates",
		"TestAgentRun_AVisibleHighWithAFencedPassVerdictStaysFail",
	)
}

func TestC2495_003_AReportWithNoVerdictAtAllStillFails(t *testing.T) {
	runAgentRunTests(t,
		"TestAgentRun_AReportWithNoVerdictInOrOutOfAFenceStillFails",
		"TestAgentRun_NoStatedVerdictWithNoBlockingFindingEmitsAParseDefect",
		"TestReportVerdict_TheParseDefectNeedsNoStatedVerdictAndNoBlockingFinding",
	)
}

func TestC2495_004_AFullyFencedReportWithABlockingFindingStaysFail(t *testing.T) {
	runAgentRunTests(t,
		"TestAgentRun_AFullyFencedReportWithABlockingFindingStaysFail",
		"TestReportVerdict_AFencedBlockingFindingBlocksTheVerdictThatTheFenceStates",
		"TestReportVerdict_AVisibleVerdictKeepsAFencedExampleFindingUncounted",
	)
}
