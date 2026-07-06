//go:build acs

// Package cycle550 materialises the cycle-550 acceptance criteria for this
// fleet lane's sole `## top_n` task per triage-report.md:
//
//	supervisor-continuous-lane-keeping (L5, "the ceiling-keeper" of the
//	fleet-width architecture) — convert the wave-synchronized scheduler into
//	a rolling lane pool: the supervisor maintains target width =
//	fleet.count, and on any lane exit (PASS or FAIL) immediately selects and
//	dispatches the next disjoint pending task as a replacement lane instead
//	of waiting for the wave barrier. Config-gated via
//	`policy.fleet.scheduling: pool|wave` (default "wave" preserves today's
//	behavior byte-identically).
//
// Per triage-report.md's "Fleet scope note", the scout-proposed
// eliminate-sequential-fallback-min-width-lane, memo-phase-routing-restore,
// and fuzz-parser-surfaces tasks are OUT OF SCOPE for this lane (assigned to
// sibling concurrent cycles) and are NOT predicated here (AC-Materialization
// Contract R9.3: predicates bind ONLY to triage-committed `## top_n` work).
//
// Predicate strategy (mirrors cycle547/549): BEHAVIORAL predicates drive the
// system under test through its in-package RED tests via subprocess
// `go test`, asserting a non-degenerate pass (requireTestsRan closes the
// cycle-85 "no tests to run" trap) — never a source grep. The in-package
// tests were authored by the TDD engineer this cycle:
//
//	internal/fleet/pool_test.go                       (new RunPool contract)
//	internal/policy/fleet_config_scheduling_test.go   (new scheduling knob)
//
// RED today: both packages fail to BUILD (RunPool/PoolConfig/PoolTransition
// and FleetPolicy.Scheduling/FleetConfig.Scheduling are all undefined) — a
// subprocess `go test` on either package exits non-zero with a compile
// error, which is exactly what every predicate below asserts against. The
// Builder implements production code ONLY (the seams named in those files
// and their header doc-comments); it must not modify the tests.
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

// runGoTest runs `go test` on pkg filtered by runFilter, returning combined
// output + exit code. Behavioral predicates invoke the system under test
// through its own in-package tests — no source-grep gaming.
func runGoTest(t *testing.T, runFilter, pkg string) (out string, code int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", runFilter, pkg)
	return stdout + "\n" + stderr, code
}

// requireTestsRan closes the degenerate-predicate trap: `go test -run X` with
// no matching test exits 0 with "no tests to run", which would green a
// predicate on unwritten (or renamed) work. A build failure (today's RED
// state) also does not print "=== RUN" lines, so it correctly falls through
// to the caller's own code!=0 check rather than satisfying this helper.
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

// TestC550_001_RunPool_BackfillsWhileSiblingStillRunning (AC1: "kill/fail
// one running lane -> a replacement lane dispatches within one scheduler
// tick while the sibling lane is STILL RUNNING") plus the selection-priority
// refinement from the fix's design (highest-Priority disjoint candidate
// wins). Drives internal/fleet/pool_test.go. RED today: RunPool/PoolConfig
// undefined (package fleet test build fails).
func TestC550_001_RunPool_BackfillsWhileSiblingStillRunning(t *testing.T) {
	out, code := runGoTest(t,
		"TestRunPool_BackfillsReplacementWhileSiblingLaneStillRunning|TestRunPool_BackfillPrefersHighestPriorityDisjointCandidate",
		fleetPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("rolling lane-pool backfill is red (exit=%d) — fleet.RunPool missing or does not backfill while a sibling lane is still running\n%s", code, out)
	}
}

// TestC550_002_RunPool_DisjointnessNeverCollidesAndDrainsBacklog (AC4: "No
// disjoint supply -> pool shrinks to 1 ... and emits the width signal") plus
// the zero-backlog edge (the pool-mode analogue of the wave path's D1 empty-
// plan guard). Drives internal/fleet/pool_test.go. RED today: same build
// failure as above.
func TestC550_002_RunPool_DisjointnessNeverCollidesAndDrainsBacklog(t *testing.T) {
	out, code := runGoTest(t,
		"TestRunPool_CollidingFilesNeverCoRunButAllEventuallyDispatch|TestRunPool_EmptyBacklogIdlesCleanlyNoLaunchCalls",
		fleetPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("rolling lane-pool disjointness/drain contract is red (exit=%d) — colliding-file todos must never co-run, and every backlog item (or none, for an empty backlog) must still be dispatched\n%s", code, out)
	}
}

// TestC550_003_RunPool_EmitsLiveTargetTelemetry (AC2: "Realized-width
// telemetry: pool logs 'lanes live: N/target' transitions") — pins the DATA
// contract (PoolTransition) this package emits; the caller's log-line
// formatting and the soak-batch time-at-target-width comparison are
// test-report.md manual+checklist items, not mechanically unit-testable
// here. Drives internal/fleet/pool_test.go. RED today: PoolTransition
// undefined (package fleet test build fails).
func TestC550_003_RunPool_EmitsLiveTargetTelemetry(t *testing.T) {
	out, code := runGoTest(t, "TestRunPool_EmitsShrinkAndRecoveryTransitions", fleetPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("rolling lane-pool telemetry is red (exit=%d) — RunPool must report live/target transitions, including RECOVERY back to target width after a backfill\n%s", code, out)
	}
}

// TestC550_004_FleetConfig_SchedulingClosedVocab (AC5: "policy
// fleet.scheduling selects pool|wave; wave mode preserves today's behavior
// byte-identically") — pins the new closed-vocab config knob and its
// non-interference with the rest of FleetConfig's resolution. Drives
// internal/policy/fleet_config_scheduling_test.go. RED today:
// FleetPolicy.Scheduling/FleetConfig.Scheduling undefined (package policy
// test build fails).
func TestC550_004_FleetConfig_SchedulingClosedVocab(t *testing.T) {
	out, code := runGoTest(t,
		"TestFleetConfig_SchedulingClosedVocab|TestFleetConfig_SchedulingAbsentPreservesRestOfConfigByteIdentical",
		policyPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("fleet.scheduling closed-vocab config knob is red (exit=%d) — FleetConfig must resolve wave (default)/pool, fail unknown values safe to wave with a warning, and leave Count/Concurrency/MinLanes/PlanSource resolution untouched\n%s", code, out)
	}
}
