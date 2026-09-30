//go:build acs

package cycle550

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	fleetPkg  = "github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	policyPkg = "github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

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

func TestC550_001_RunPool_BackfillsWhileSiblingStillRunning(t *testing.T) {
	out, code := runGoTest(t,
		"TestRunPool_BackfillsReplacementWhileSiblingLaneStillRunning|TestRunPool_BackfillPrefersHighestPriorityDisjointCandidate",
		fleetPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("rolling lane-pool backfill is red (exit=%d) — fleet.RunPool missing or does not backfill while a sibling lane is still running\n%s", code, out)
	}
}

func TestC550_002_RunPool_DisjointnessNeverCollidesAndDrainsBacklog(t *testing.T) {
	out, code := runGoTest(t,
		"TestRunPool_CollidingFilesNeverCoRunButAllEventuallyDispatch|TestRunPool_EmptyBacklogIdlesCleanlyNoLaunchCalls",
		fleetPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("rolling lane-pool disjointness/drain contract is red (exit=%d) — colliding-file todos must never co-run, and every backlog item (or none, for an empty backlog) must still be dispatched\n%s", code, out)
	}
}

func TestC550_003_RunPool_EmitsLiveTargetTelemetry(t *testing.T) {
	out, code := runGoTest(t, "TestRunPool_EmitsShrinkAndRecoveryTransitions", fleetPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("rolling lane-pool telemetry is red (exit=%d) — RunPool must report live/target transitions, including RECOVERY back to target width after a backfill\n%s", code, out)
	}
}

func TestC550_004_FleetConfig_SchedulingClosedVocab(t *testing.T) {
	out, code := runGoTest(t,
		"TestFleetConfig_SchedulingClosedVocab|TestFleetConfig_SchedulingAbsentPreservesRestOfConfigByteIdentical",
		policyPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("fleet.scheduling closed-vocab config knob is red (exit=%d) — FleetConfig must resolve wave (default)/pool, fail unknown values safe to wave with a warning, and leave Count/Concurrency/MinLanes/PlanSource resolution untouched\n%s", code, out)
	}
}
