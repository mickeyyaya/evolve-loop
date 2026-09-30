//go:build acs

package cycle552

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

func runGoTest(t *testing.T, runFilter, pkg string) (out string, code int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", runFilter, pkg)
	return stdout + "\n" + stderr, code
}

func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten or renamed")
		return
	}
	if got := strings.Count(out, "=== RUN"); got < min {
		t.Logf("only %d test(s) ran (want >= %d) — output:\n%s", got, min, out)
	}
}

func TestC552_001_MinWidthRepair_GuardConditionGatesEligibility(t *testing.T) {
	out, code := runGoTest(t, "TestMinWidthRepair_GuardNotMetNeverInvokesLauncher", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("minWidthRepair must gate on fleetCfg.Count>1 && waveCfg.Count<=1 and never touch preflight/planFn/launcher when ineligible (exit=%d)\n%s", code, out)
	}
}

func TestC552_002_MinWidthRepair_DispatchesOneIsolatedLaneAndContinues(t *testing.T) {
	out, code := runGoTest(t, "TestMinWidthRepair_GuardMetDispatchesOneIsolatedLaneAndSignalsContinue", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("minWidthRepair must dispatch exactly one isolated lane through the injected launcher and report handled=true (exit=%d)\n%s", code, out)
	}
}

func TestC552_003_MinWidthRepair_EmptyBacklogPreservesTrueSequentialFallback(t *testing.T) {
	out, code := runGoTest(t, "TestMinWidthRepair_EligibleButEmptyBacklogFallsBackToSequential", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("an eligible-but-empty backlog must never invoke the launcher and must WARN the empty-backlog message (exit=%d)\n%s", code, out)
	}
}

func TestC552_004_MinWidthRepair_ForceDispatchErrorNeverSilentlySwallowed(t *testing.T) {
	out, code := runGoTest(t, "TestMinWidthRepair_ForceDispatchErrorSurfacesAndFallsBack", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("a forceOneLaneDispatch error must surface in the WARN message and report handled=false (exit=%d)\n%s", code, out)
	}
}
