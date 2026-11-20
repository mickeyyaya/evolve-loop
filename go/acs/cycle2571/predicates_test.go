//go:build acs

package cycle2571

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func integrationTestPasses(t *testing.T, name string) {
	t.Helper()
	args := []string{"-C", acsassert.GoModuleDir(t), "test", "-count=1", "-v", "-run", "^" + name + "$", "./internal/integration"}
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", args...)
	passed := err == nil && code == 0 && strings.Contains(stdout, "--- PASS: "+name+" (")
	if !passed {
		t.Errorf("RED: %s failed (exit %d)\n%s%s", name, code, acsassert.OutputTail(stdout, 15), acsassert.OutputTail(stderr, 5))
	}
}

func TestC2571_001_AGreenRunWinsOverANewerCancelledRun(t *testing.T) {
	integrationTestPasses(t, "TestCheckMain_AGreenRunWinsOverANewerCancelledRun")
}

func TestC2571_002_ARedRunOfTheSHAStaysRed(t *testing.T) {
	integrationTestPasses(t, "TestCheckMain_ARedRunOfTheSHAStaysRed")
}

func TestC2571_003_OnlyCancelledRunsLeaveMainNotGreen(t *testing.T) {
	integrationTestPasses(t, "TestCheckMain_OnlyCancelledRunsLeaveMainNotGreen")
}

func TestC2571_004_NoRunOfTheSHAIsStillNoRun(t *testing.T) {
	integrationTestPasses(t, "TestCheckMain_NoRunOfTheSHAIsStillNoRun")
}
