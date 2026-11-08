//go:build acs

package cycle2229

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

func runCoreTests(t *testing.T, names ...string) {
	t.Helper()
	moduleDir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatal(err)
	}
	pattern := "^(" + strings.Join(names, "|") + ")$"
	out, errOut, code, runErr := acsassert.SubprocessOutput("go", "-C", moduleDir, "test", "-count=1", "-v", "-run", pattern, "./internal/core")
	if runErr != nil || code != 0 {
		t.Fatalf("RED: go test ./internal/core -run %s exit=%d err=%v\n%s", pattern, code, runErr, acsassert.OutputTail(out+errOut, 25))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass", name)
		}
	}
}

func adrItemNine(t *testing.T) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "adr", "0146-*.md"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("ADR-0146 matches=%v err=%v, want exactly one file", matches, err)
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	var item string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "9. ") {
			item = line
		}
	}
	if item == "" {
		t.Fatalf("ADR-0146 has no item 9 on the end of the ladder")
	}
	return item
}

func TestC2229_001_TheEndOfTheLadderRoutesToTheRedesignThroughAskLadderAndRunCycle(t *testing.T) {
	runCoreTests(t,
		"TestAskLadder_TheEndOfTheLadderRoutesToTheRedesign",
		"TestAskLadder_ASpecStopRoutesToTheRedesign",
		"TestRunCycle_AFailedLadderRoutesOnceToTheRedesignRung")
}

func TestC2229_002_NoRedesignBeforeTheEndAndNoSecondRedesignAfterIt(t *testing.T) {
	runCoreTests(t,
		"TestAskLadder_NoRedesignRouteBeforeTheEndOfTheLadder",
		"TestAskLadder_ASecondEndOfTheLadderGoesToTheOperatorWithNoSecondRedesign",
		"TestRunCycle_APassingBuildNeverRoutesToTheRedesignRung")
}

func TestC2229_003_TheRedesignCodeIsRegisteredAndTheRecordNamesIt(t *testing.T) {
	module, ok := signalcenter.IsRegistered(redesignCode)
	if !ok || module != signalcenter.ModuleLadder {
		t.Fatalf("RED: %s registered=%v module=%q, want the ladder module", redesignCode, ok, module)
	}
	item := adrItemNine(t)
	for _, want := range []string{"redesign", string(redesignCode), "once", "second", "operator"} {
		if !strings.Contains(item, want) {
			t.Errorf("RED: ADR-0146 item 9 lacks %q, so the record and the code disagree: %s", want, item)
		}
	}
}

func TestC2229_004_TheRecordNoLongerEndsTheLadderAtTheOperator(t *testing.T) {
	item := adrItemNine(t)
	if strings.Contains(item, "After R3 the ladder stops") {
		t.Errorf("RED: ADR-0146 item 9 still says the ladder stops after R3: %s", item)
	}
	if idx, second := strings.Index(item, "operator"), strings.Index(item, "second"); idx >= 0 && second > idx {
		t.Errorf("RED: ADR-0146 item 9 sends the subject to the operator before the second end: %s", item)
	}
}
