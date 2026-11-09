//go:build acs

package cycle2246

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runGoTest(t *testing.T, pkg string, names ...string) {
	t.Helper()
	moduleDir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatal(err)
	}
	pattern := "^(" + strings.Join(names, "|") + ")$"
	out, errOut, code, runErr := acsassert.SubprocessOutput("go", "-C", moduleDir, "test", "-count=1", "-v", "-run", pattern, pkg)
	if runErr != nil || code != 0 {
		t.Fatalf("RED: go test %s -run %s exit=%d err=%v\n%s", pkg, pattern, code, runErr, acsassert.OutputTail(out+errOut, 30))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass in %s", name, pkg)
		}
	}
}

func TestC2246_001_TheQuotaWallExitSignalNamesTheQuotaNotTheUnknownPrompt(t *testing.T) {
	runGoTest(t, "./internal/bridge/launchoutcome",
		"TestClassify_ARouterUsageLimitExitNamesTheQuotaNotTheUnknownPrompt",
		"TestClassify_EscalationPatternRidesTheCause")
}

func TestC2246_002_AnEmptyOrUnnamedPaneStillReadsAsTheUnknownPrompt(t *testing.T) {
	runGoTest(t, "./internal/bridge/launchoutcome", "TestClassify_AnUnknownPromptExitKeepsTheUnknownPromptSignal")
}
