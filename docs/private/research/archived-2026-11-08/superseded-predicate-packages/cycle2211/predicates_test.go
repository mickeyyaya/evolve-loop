//go:build acs

package cycle2211

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const redesignCode signalcenter.Code = "LADDER_REDESIGN"

func runCoreTests(t *testing.T, pattern string) string {
	t.Helper()
	moduleDir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatal(err)
	}
	out, errOut, code, runErr := acsassert.SubprocessOutput("go", "-C", moduleDir, "test", "-count=1", "-v", "-run", pattern, "./internal/core")
	if runErr != nil || code != 0 {
		t.Fatalf("RED: go test ./internal/core -run %s exit=%d err=%v\n%s", pattern, code, runErr, acsassert.OutputTail(out+errOut, 25))
	}
	return out
}

func requirePassed(t *testing.T, out string, names ...string) {
	t.Helper()
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass", name)
		}
	}
}

func TestC2211_001_TheEndOfTheLadderRoutesToTheRedesignThroughAskLadder(t *testing.T) {
	out := runCoreTests(t, "TestAskLadder_TheEndOfTheLadderRoutesToTheRedesign|TestAskLadder_ASpecStopRoutesToTheRedesign|TestAskLadder_TheCodeOfTheRedesignRouteIsRegisteredUnderTheLadderModule")
	requirePassed(t, out,
		"TestAskLadder_TheEndOfTheLadderRoutesToTheRedesign",
		"TestAskLadder_ASpecStopRoutesToTheRedesign",
		"TestAskLadder_TheCodeOfTheRedesignRouteIsRegisteredUnderTheLadderModule")
}

func TestC2211_002_NoRedesignBeforeTheEndAndNoSecondRedesignAfterIt(t *testing.T) {
	out := runCoreTests(t, "TestAskLadder_NoRedesignRouteBeforeTheEndOfTheLadder|TestAskLadder_ASecondEndOfTheLadderGoesToTheOperatorWithNoSecondRedesign")
	requirePassed(t, out,
		"TestAskLadder_NoRedesignRouteBeforeTheEndOfTheLadder",
		"TestAskLadder_ASecondEndOfTheLadderGoesToTheOperatorWithNoSecondRedesign")
}

func TestC2211_003_TheRecordAndTheCodeAgreeOnTheRedesignRoute(t *testing.T) {
	module, ok := signalcenter.IsRegistered(redesignCode)
	if !ok || module != signalcenter.ModuleLadder {
		t.Fatalf("RED: %s registered=%v module=%q, want the ladder module", redesignCode, ok, module)
	}
	root := acsassert.RepoRoot(t)
	matches, err := filepath.Glob(filepath.Join(root, "docs", "architecture", "adr", "0146-*.md"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("ADR-0146 matches=%v err=%v, want exactly one file", matches, err)
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	var endOfLadder string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "9. ") {
			endOfLadder = line
		}
	}
	if endOfLadder == "" {
		t.Fatalf("ADR-0146 has no item 9 on the end of the ladder")
	}
	if !strings.Contains(endOfLadder, "redesign") || !strings.Contains(endOfLadder, string(redesignCode)) {
		t.Errorf("RED: ADR-0146 item 9 does not route the end of the ladder to the redesign rung with %s: %s", redesignCode, endOfLadder)
	}
	if strings.Contains(endOfLadder, "the operator gets the dossier") && !strings.Contains(endOfLadder, "second") {
		t.Errorf("RED: ADR-0146 item 9 still ends the ladder at the operator: %s", endOfLadder)
	}
}
