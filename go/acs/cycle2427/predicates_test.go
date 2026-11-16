//go:build acs

package cycle2427

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cmdPackage   = "./cmd/evolve"
	oversightSym = "internal/orchestrator.(*Oversight).Run"
	waveDoc      = "docs/architecture/wave-orchestrator.md"
)

func goTest(t *testing.T, names []string) (string, int) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	out, errOut, code, _ := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-v", "-timeout", "300s", "-run", "^("+strings.Join(names, "|")+")$", cmdPackage)
	return out + errOut, code
}

func requireGreen(t *testing.T, names ...string) {
	t.Helper()
	out, code := goTest(t, names)
	if code != 0 {
		t.Fatalf("RED: go test %s exited %d\n%s", cmdPackage, code, acsassert.OutputTail(out, 40))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass in %s", name, cmdPackage)
		}
	}
}

func TestC2427_001_TheLoopConstructsAndRunsTheOversightAndStopsItWithThePool(t *testing.T) {
	requireGreen(t, "TestOrchestratedRun_ConstructsAndRunsOversightAndStopsItWithThePool")
}

func TestC2427_002_ACleanLandingMovesTheAdaptiveWindowAtEnforce(t *testing.T) {
	requireGreen(t, "TestOrchestratedRun_ACleanLandingMovesTheAdaptiveWindowAtEnforce")
}

func TestC2427_003_ALandingAtShadowMovesNothing(t *testing.T) {
	requireGreen(t, "TestOrchestratedRun_ALandingAtShadowIsRecordedAndMovesNothing")
}

func TestC2427_004_AnEventSourceThatCannotOpenDoesNotStopThePool(t *testing.T) {
	requireGreen(t, "TestOrchestratedRun_AnEventSourceThatCannotOpenLeavesThePoolRunningWithAWarning")
}

func TestC2427_005_TheOversightStopSignalReachesThePoolIntake(t *testing.T) {
	requireGreen(t, "TestOrchestratedRun_TheOversightHaltReachesThePoolIntakeWhenCellAdmissionLoads")
}

func TestC2427_006_TheEvolveBinaryLinksTheOversightRunLoop(t *testing.T) {
	root := acsassert.RepoRoot(t)
	binary := filepath.Join(t.TempDir(), "evolve")
	if _, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "build", "-o", binary, cmdPackage); err != nil || code != 0 {
		t.Fatalf("go build %s exited %d: %v\n%s", cmdPackage, code, err, errOut)
	}
	symbols, errOut, code, err := acsassert.SubprocessOutput("go", "tool", "nm", binary)
	if err != nil || code != 0 {
		t.Fatalf("go tool nm exited %d: %v\n%s", code, err, errOut)
	}
	if !strings.Contains(symbols, oversightSym) {
		t.Errorf("RED: the linker dropped %s: no production code reaches the oversight loop", oversightSym)
	}
}

func TestC2427_007_TheDefaultEventSourcePathKeepsTheExistingPoolRunsGreen(t *testing.T) {
	requireGreen(t, "TestOrchestratedRun_TheResupplyFeedsTheFreedSlot", "TestOrchestratorPool_WiresTheEnforceKeysAndTheFileRuleWithoutAGraph", "TestOrchestratorPool_AdmitsByCellWhenTheModuleGraphLoads")
}

// acs-predicate: config-check
func TestC2427_008_TheWaveOrchestratorDocNoLongerSaysTheOversightIsUnwired(t *testing.T) {
	doc := filepath.Join(acsassert.RepoRoot(t), waveDoc)
	for _, stale := range []string{"The loop wiring of the oversight waits", "O6 is not built", "The caller that moves the window is not built"} {
		if !acsassert.FileNotContains(t, doc, stale) {
			t.Errorf("RED: %s still says %q", waveDoc, stale)
		}
	}
	if !acsassert.FileContains(t, doc, "orchestratedRun") {
		t.Errorf("RED: %s does not name orchestratedRun as the place that runs the oversight", waveDoc)
	}
}
