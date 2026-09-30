//go:build acs

package cycle767

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const fleetPkg = "github.com/mickeyyaya/evolve-loop/go/internal/fleet"

func runGoTest(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race %s -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pkg, name, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Fatalf("go test reported no PASS for %s (renamed or not run?)\nstdout:\n%s", name, stdout)
	}
}

func TestC767_001_SkipsConsumedTaskAndRefillsSlot(t *testing.T) {
	runGoTest(t, fleetPkg, "TestWaveDispatch_SkipsConsumedTaskAndRefillsSlot")
}

func TestC767_002_SkipsDepsUnmetTaskWithReason(t *testing.T) {
	runGoTest(t, fleetPkg, "TestWaveDispatch_SkipsDepsUnmetTaskWithReason")
}

func TestC767_003_EmptyScopeAfterGateVerdictsSkippedNotFail(t *testing.T) {
	runGoTest(t, fleetPkg, "TestBuildEmptyScope_AfterFreshnessGate_VerdictSkippedNotFail")
}

func TestC767_004_GateNegativeAndEdgeAxes(t *testing.T) {
	runGoTest(t, fleetPkg, "TestWaveDispatch_AllFresh_NoSkipNoRefill")
	runGoTest(t, fleetPkg, "TestWaveDispatch_PartialStaleScope_FiltersIdKeepsSpec")
}
