//go:build acs

package cycle2490

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

func requireBridgeTestsPass(t *testing.T, names ...string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", "-v", "-run", "^("+strings.Join(names, "|")+")$", bridgePkg)
	if code < 0 {
		t.Fatalf("go test did not launch: %v", err)
	}
	report := stdout + stderr
	if code != 0 {
		t.Errorf("RED: bridge tests %v exit %d\n%s", names, code, acsassert.OutputTail(report, 30))
		return
	}
	for _, name := range names {
		if !strings.Contains(report, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass", name)
		}
	}
}

func TestC2490_001_ARevListThatExitsOneWithPartialStdoutIsAnErrorNotEvidence(t *testing.T) {
	requireBridgeTestsPass(t,
		"TestGitEvidence_RevListExitOneWithPartialStdoutIsAnErrorNotEvidence",
		"TestGitEvidence_LogExitOneWithTheEvidenceMessageIsNotEvidence",
		"TestGitEvidence_RevParseExitOneNeverBecomesTheBaseline",
	)
}

func TestC2490_002_TheBridgeFakeGitAndItsContractSuiteAssertANonZeroExitGivesAnError(t *testing.T) {
	requireBridgeTestsPass(t,
		"TestExitingGit_ReportsTheScriptedExitCodeAndTheStdout",
		"TestGitEvidence_GitCmdContract_NonZeroExitIsAnErrorForEverySubcommand",
		"TestGitEvidence_GitCmdContract_ZeroExitKeepsTheTrimmedStdout",
	)
}
