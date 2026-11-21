//go:build acs

package cycle2582

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func requireTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	out, errOut, code, _ := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-v", "-timeout", "300s", "-run", "^("+strings.Join(names, "|")+")$", pkg)
	out += errOut
	if code != 0 {
		t.Fatalf("RED: go test %s exited %d\n%s", pkg, code, acsassert.OutputTail(out, 40))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass in %s", name, pkg)
		}
	}
}

func TestC2582_001_APoolRelaunchesAParkedLeadAfterItsLaneExited(t *testing.T) {
	requireTestsPass(t, "./internal/fleet", "TestPoolResupply_RelaunchesAParkedLeadAfterItsLaneExited")
}

func TestC2582_002_AnUnmarkedKnownTodoStaysDeduped(t *testing.T) {
	requireTestsPass(t, "./internal/fleet", "TestPoolResupply_KeepsAnUnmarkedKnownTodoDeduped")
}

func TestC2582_003_AMarkedTodoIsNeverLaunchedWhileItsLaneRunsOrIsPending(t *testing.T) {
	requireTestsPass(t, "./internal/fleet",
		"TestPoolResupply_NeverRelaunchesAMarkedTodoWhileItsLaneRuns",
		"TestPoolResupply_NeverRelaunchesAMarkedTodoWhileItsLaneIsPending")
}

func TestC2582_004_AReleasedMainRedScopeIsLaunchedAgainThroughThePlannerAndTheProductionPool(t *testing.T) {
	requireTestsPass(t, "./cmd/evolve", "TestProductionPoolPlanFn_AReleasedMainRedStopRelaunchesTheParkedLeadThePoolAlreadyLaunched")
}
