//go:build acs

package cycle2451

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	agentcmdPkg  = "./internal/cli/agentcmd"
	warnTestFile = "go/internal/cli/agentcmd/agent_fence_warn_test.go"
)

var (
	warnLineTests = []string{
		"TestAgentRun_ASeatThatCreatesAFileOutsideItsGrantPrintsOneWarnLineWithThePathAndTheNextStep",
		"TestAgentRun_TheFenceWarnLineNamesEveryRevertedPathOfTheRun",
		"TestAgentRun_ASeatThatTampersWithAnExistingFileOutsideItsGrantGetsAWarnLineThatNamesTheRestoredPath",
	}
	distinctCodeTests = []string{
		"TestAgentRun_ASeatThatCreatesAFileOutsideItsGrantExitsWithACodeNoOtherOutcomeUses",
		"TestAgentRun_TheUsageDocumentsTheFenceRevertExitCode",
	}
	correctedGrantTests = []string{
		"TestAgentRun_TheConsoleRerunsASeatWithTheCorrectedGrantAndTheRunPassesWithoutAWarnLine",
		"TestAgentRun_ASeatThatStaysInsideItsGrantPrintsNoWarnLineAndExitsZero",
	}
)

func eachPasses(t *testing.T, names []string) {
	t.Helper()
	moduleDir := acsassert.GoModuleDir(t)
	for _, name := range names {
		stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", moduleDir, "test", "-count=1", "-v", "-run", "^"+name+"$", agentcmdPkg)
		if err != nil || code != 0 {
			t.Errorf("%s exited %d (%v):\n%s", name, code, err, acsassert.OutputTail(stdout+stderr, 30))
			continue
		}
		if !strings.Contains(stdout, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass; a -run that matches nothing exits 0:\n%s", name, acsassert.OutputTail(stdout, 15))
		}
	}
}

func TestC2451_001_AFenceRevertPrintsAWarnLineThatNamesTheRevertedPaths(t *testing.T) {
	eachPasses(t, warnLineTests)
}

func TestC2451_002_AFenceRevertExitsWithADistinctNonZeroCodeThatTheUsageDocuments(t *testing.T) {
	eachPasses(t, distinctCodeTests)
}

func TestC2451_003_TheConsoleRerunsWithACorrectedGrantAndASeatInsideItsGrantStaysQuiet(t *testing.T) {
	eachPasses(t, correctedGrantTests)
}

func TestC2451_004_TheNewTestsAreShippableAndListedSoTheyCannotBeDroppedToGoGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, warnTestFile)) {
		t.Fatalf("%s is missing on disk", warnTestFile)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", warnTestFile); code == 0 {
		t.Fatalf("%s is gitignored and would be dropped at ship", warnTestFile)
	}
	listed, stderr, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-list", "^TestAgentRun_", agentcmdPkg)
	if err != nil || code != 0 {
		t.Fatalf("go test -list exited %d (%v): %s", code, err, acsassert.OutputTail(stderr, 20))
	}
	var want []string
	for _, group := range [][]string{warnLineTests, distinctCodeTests, correctedGrantTests} {
		want = append(want, group...)
	}
	for _, name := range want {
		if !strings.Contains("\n"+listed, "\n"+name+"\n") {
			t.Errorf("%s is not listed by go test -list in %s", name, agentcmdPkg)
		}
	}
}
