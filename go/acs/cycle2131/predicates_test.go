//go:build acs

package cycle2131

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const agentcmdPackage = "./internal/cli/agentcmd"

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runAgentcmdTests(t *testing.T, names ...string) bool {
	t.Helper()
	return acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     goDir(t),
		Package: agentcmdPackage,
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
	})
}

func TestC2131_001_AgentRunRefusesAWritableThatNamesADirectoryAndStartsNoSeat(t *testing.T) {
	runAgentcmdTests(t, "TestAgentRun_TheBuildSeatRefusesAWritableThatNamesADirectory")
}

func TestC2131_002_AgentRunStillAcceptsAWritableFileThatDoesNotExistYet(t *testing.T) {
	runAgentcmdTests(t, "TestAgentRun_TheBuildSeatAcceptsAWritableThatIsAFileOrNotYetThere")
}

func TestC2131_003_TheRefusalOfADirectoryKeepsTheOtherBuildSeatRefusals(t *testing.T) {
	runAgentcmdTests(t, "TestAgentRun_TheBuildSeatRefusals", "TestAgentRun_TheBuildSeatRefusesAWritableThatNamesADirectory")
}

func TestC2131_004_TheUsageOfAgentRunStatesTheDirectoryRule(t *testing.T) {
	runAgentcmdTests(t, "TestAgentRun_TheUsageStatesThatWritableNamesAFileNeverADirectory")
}

// acs-predicate: config-check
func TestC2131_005_TheAgentRunContractDocStatesTheDirectoryRule(t *testing.T) {
	doc := filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "agent-run.md")
	if !acsassert.LineContainsAll(doc, "--writable", "directory") {
		t.Errorf("RED: %s has no line that names --writable and a directory", doc)
	}
}
