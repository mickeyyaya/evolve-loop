//go:build acs

package cycle2452

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const packageNote = "docs/architecture/packages/internal-orchestrator.md"

func runNamedTest(t *testing.T, pkg, name string, extra ...string) (string, bool) {
	t.Helper()
	args := append([]string{"test", "-count=1", "-v", "-run", "^" + name + "$"}, extra...)
	args = append(args, pkg)
	cmd := exec.Command("go", args...)
	cmd.Dir = acsassert.GoModuleDir(t)
	raw, err := cmd.CombinedOutput()
	out := string(raw)
	if err != nil {
		if _, isExit := err.(*exec.ExitError); !isExit {
			t.Fatalf("go test did not run: %v", err)
		}
		return out, false
	}
	return out, strings.Contains(out, "--- PASS: "+name+" ")
}

func TestC2452_001_TheProductionLocatorNeverReturnsTheCycleOfAnEarlierLife(t *testing.T) {
	out, passed := runNamedTest(t, "./cmd/evolve", "TestLaneLocator_ANewLifeOfATodoNeverTakesTheCycleOfItsEarlierLife")
	if !passed {
		t.Errorf("RED: the Locate port that orchestratorPool wires returns the cycle of an earlier life of the same todo\n%s", acsassert.OutputTail(out, 12))
	}
}

func TestC2452_002_TheProductionLocatorStillFindsFirstLivesPerTodo(t *testing.T) {
	out, passed := runNamedTest(t, "./cmd/evolve", "TestLaneLocator_AFirstLifeStillFindsItsCycleAndAnotherTodoKeepsItsOwn")
	if !passed {
		t.Errorf("RED: the production locator no longer finds the cycle of a first life\n%s", acsassert.OutputTail(out, 12))
	}
}

func TestC2452_003_TheFloorLocatorIsSafeForTheSealAndTheOversightAtOnce(t *testing.T) {
	out, passed := runNamedTest(t, "./internal/orchestrator", "TestRunsLocator_ASealAndTheOversightMayLocateAtOnce", "-race")
	if !passed {
		t.Errorf("RED: RunsLocator keeps unlocked maps while Pool.recover and the oversight both call it\n%s", acsassert.OutputTail(out, 14))
	}
}

// acs-predicate: config-check
func TestC2452_004_ThePackageNoteNamesTheLocatePortThatProductionWires(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, packageNote))
	if err != nil {
		t.Fatal(err)
	}
	var namesRunsLocatorAsLocate, namesLaneCycleAsLocate, claimsSingleCaller bool
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, "`Locate`") && strings.Contains(line, "`RunsLocator`") {
			namesRunsLocatorAsLocate = true
		}
		if strings.Contains(line, "`Locate` port") && strings.Contains(line, "`orchestrator.LaneCycle`") {
			namesLaneCycleAsLocate = true
		}
		if strings.Contains(line, "`RunsLocator`") && strings.Contains(line, "only caller") {
			claimsSingleCaller = true
		}
	}
	if !namesRunsLocatorAsLocate {
		t.Errorf("RED: %s has no line that ties the `Locate` port to `RunsLocator`", packageNote)
	}
	if namesLaneCycleAsLocate {
		t.Errorf("RED: %s still names `orchestrator.LaneCycle` as the `Locate` port that production wires", packageNote)
	}
	if claimsSingleCaller {
		t.Errorf("RED: %s still says the oversight is the only caller of `RunsLocator`; Pool.recover calls it too", packageNote)
	}
}
