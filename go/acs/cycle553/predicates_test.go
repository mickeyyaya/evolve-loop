//go:build acs

// Package cycle553 materialises the cycle-553 acceptance criteria for this
// fleet lane's SOLE `## top_n` task per triage-report.md:
//
//	supervisor-continuous-lane-keeping — wire the cycle-550 fleet.RunPool
//	rolling-lane-pool scheduler into the actual loop dispatcher behind the
//	existing policy.fleet.scheduling knob (add the missing "pool" branch;
//	RunPool currently has zero call sites outside its own package), backfilling
//	a replacement lane on any lane exit while sibling lanes still run, honoring
//	L4 min-width and per-run cycle-state isolation.
//
// Per the AC-Materialization Contract (R9.3 "predicates bind ONLY to triage-
// committed work"), this package predicates ONLY that item. The cycle-553
// scout-report.md proposed two OTHER tasks (acsassert-hermetic-coverage-floor,
// wave-seed-min-width-one-lane); triage-report.md explicitly DROPPED both as
// sibling-lane/out-of-fleet-scope, so they get NO predicate here.
//
// Predicate strategy (behavioral-via-subprocess, the cycle-549 precedent — never
// a source grep): the dispatcher functions under test (shouldRunPool,
// dispatchPoolIteration) live in `package main` (cmd/evolve), which a leaf ACS
// package cannot import. So each predicate drives `go test ./cmd/evolve -run
// <TestName>` as a subprocess over the REAL compiled dispatcher + the in-package
// behavioral tests the TDD engineer authored this cycle
// (cmd/evolve/cmd_loop_pool_test.go), asserting (a) the targeted tests actually
// ran — closes the cycle-85 "no tests to run" degenerate trap — and (b) they
// passed (exit 0), i.e. the wiring exists and behaves. Before the Builder wires
// it, cmd/evolve's test build fails to compile (undefined shouldRunPool /
// dispatchPoolIteration), so `go test` exits non-zero and every predicate is RED
// for the right reason.
//
// In-package behavioral tests these predicates gate on:
//
//	cmd/evolve/cmd_loop_pool_test.go
//	  TestShouldRunPool_GateTable
//	  TestShouldRunWaveAndPool_MutuallyExclusive
//	  TestDispatchPoolIteration_BackfillsReplacementWhileSiblingStillRunning
//	  TestDispatchPoolIteration_EmptyBacklogStaysFalseNoLaunch
//	  TestDispatchPoolIteration_WaveConfigInertNoLaunch
//	  TestDispatchPoolIteration_PreflightRefusalNeverPlansNorLaunches
//
// The Builder's role: add shouldRunPool / dispatchPoolIteration / poolPlanFn to
// package main and wire dispatchPoolIteration into cmd_loop.go's batch loop
// (pool branch selected before the wave branch when shouldRunPool fires). Builder
// must NOT modify the test files.
package cycle553

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

// runDispatcherTest drives `go test ./cmd/evolve -run <filter>` over the REAL
// compiled dispatcher and returns the combined output + exit code. A subprocess
// (not a source read) is the load-bearing assertion, per the predicate-quality
// rule: it exercises the actual shouldRunPool/dispatchPoolIteration code.
func runDispatcherTest(t *testing.T, runFilter string) (out string, code int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", runFilter, cmdEvolvePkg)
	return stdout + "\n" + stderr, code
}

// requireRanAndGreen fails the predicate unless the -run filter matched at least
// `min` tests (guards the cycle-85 "no tests to run" degenerate pass) and the
// package exited 0 with no `--- FAIL`.
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

// TestC553_001_ShouldRunPoolGate_SelectsPoolOnlyForPoolScheduling (AC1): the
// dispatcher gate `shouldRunPool` fires exactly for a fleet (Count>1, triage)
// whose resolved Scheduling is "pool", and stays off for wave/default/single-
// lane/manual configs — so policy.fleet.scheduling=="pool" is what routes the
// loop to the rolling pool, and the unsoaked mode is never entered by accident.
func TestC553_001_ShouldRunPoolGate_SelectsPoolOnlyForPoolScheduling(t *testing.T) {
	out, code := runDispatcherTest(t, "^TestShouldRunPool_GateTable$")
	requireRanAndGreen(t, out, code, 1)
}

// TestC553_002_WaveAndPoolGates_MutuallyExclusive (AC2 + no-regression): the
// wave and pool dispatch gates are disjoint over every config (no double
// dispatch), a "pool" fleet takes pool-not-wave, and a default/"wave" fleet
// keeps the shipped wave path untouched.
func TestC553_002_WaveAndPoolGates_MutuallyExclusive(t *testing.T) {
	out, code := runDispatcherTest(t,
		"^TestShouldRunWaveAndPool_MutuallyExclusive$|^TestDispatchPoolIteration_WaveConfigInertNoLaunch$")
	requireRanAndGreen(t, out, code, 2)
}

// TestC553_003_DispatchPoolIteration_WiresRunPoolBackfill (AC3, the core wiring
// proof): dispatchPoolIteration drives the backlog through fleet.RunPool such
// that a replacement lane backfills the instant a lane exits while a sibling is
// still running — the defining rolling-pool behavior a wave-barrier or no-op
// dispatcher cannot exhibit.
func TestC553_003_DispatchPoolIteration_WiresRunPoolBackfill(t *testing.T) {
	out, code := runDispatcherTest(t,
		"^TestDispatchPoolIteration_BackfillsReplacementWhileSiblingStillRunning$")
	requireRanAndGreen(t, out, code, 1)
}

// TestC553_004_DispatchPoolIteration_EmptyBacklogFallsBack (AC4, anti-no-op): an
// empty pool backlog reports ran=false with launch never invoked, so the caller
// falls back rather than burning a --max-cycles iteration on nothing.
func TestC553_004_DispatchPoolIteration_EmptyBacklogFallsBack(t *testing.T) {
	out, code := runDispatcherTest(t,
		"^TestDispatchPoolIteration_EmptyBacklogStaysFalseNoLaunch$")
	requireRanAndGreen(t, out, code, 1)
}

// TestC553_005_DispatchPoolIteration_PreflightGuardsBeforeDispatch (AC5, safety
// / isolation): the pool path preserves the S3 dirty-control-plane preflight — a
// refusal surfaces a wrapped error with neither planFn nor launch invoked, so a
// pool dispatch never runs against an unsafe control plane.
func TestC553_005_DispatchPoolIteration_PreflightGuardsBeforeDispatch(t *testing.T) {
	out, code := runDispatcherTest(t,
		"^TestDispatchPoolIteration_PreflightRefusalNeverPlansNorLaunches$")
	requireRanAndGreen(t, out, code, 1)
}
