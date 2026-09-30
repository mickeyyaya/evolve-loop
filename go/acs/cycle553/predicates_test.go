//go:build acs

package cycle553

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

func runDispatcherTest(t *testing.T, runFilter string) (out string, code int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", runFilter, cmdEvolvePkg)
	return stdout + "\n" + stderr, code
}

func requireRanAndGreen(t *testing.T, out string, code, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — the wiring's behavioral tests are unwritten or renamed:\n%s", out)
		return
	}
	ran := strings.Count(out, "--- PASS") + strings.Count(out, "--- FAIL")
	if ran < min {
		t.Errorf("only %d test(s) ran, need >= %d (or cmd/evolve failed to build — the pool wiring is undefined):\n%s", ran, min, out)
		return
	}
	if code != 0 || strings.Contains(out, "--- FAIL") {
		t.Errorf("cmd/evolve dispatcher tests failed (exit=%d):\n%s", code, out)
	}
}

func TestC553_001_ShouldRunPoolGate_SelectsPoolOnlyForPoolScheduling(t *testing.T) {
	out, code := runDispatcherTest(t, "^TestShouldRunPool_GateTable$")
	requireRanAndGreen(t, out, code, 1)
}

func TestC553_002_WaveAndPoolGates_MutuallyExclusive(t *testing.T) {
	out, code := runDispatcherTest(t,
		"^TestShouldRunWaveAndPool_MutuallyExclusive$|^TestDispatchPoolIteration_WaveConfigInertNoLaunch$")
	requireRanAndGreen(t, out, code, 2)
}

func TestC553_003_DispatchPoolIteration_WiresRunPoolBackfill(t *testing.T) {
	out, code := runDispatcherTest(t,
		"^TestDispatchPoolIteration_BackfillsReplacementWhileSiblingStillRunning$")
	requireRanAndGreen(t, out, code, 1)
}

func TestC553_004_DispatchPoolIteration_EmptyBacklogFallsBack(t *testing.T) {
	out, code := runDispatcherTest(t,
		"^TestDispatchPoolIteration_EmptyBacklogStaysFalseNoLaunch$")
	requireRanAndGreen(t, out, code, 1)
}

func TestC553_005_DispatchPoolIteration_PreflightGuardsBeforeDispatch(t *testing.T) {
	out, code := runDispatcherTest(t,
		"^TestDispatchPoolIteration_PreflightRefusalNeverPlansNorLaunches$")
	requireRanAndGreen(t, out, code, 1)
}
