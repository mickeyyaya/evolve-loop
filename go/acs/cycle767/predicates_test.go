//go:build acs

// Package cycle767 materializes the cycle-767 acceptance criteria for the sole
// committed top_n task dispatch-freshness-gate (triage-report.md ## top_n;
// fleet_scope pins this lane to exactly that id — the scout's own proposals
// were left for their owning cycle, so per R9.3 no predicates bind to them).
//
// Task source: inbox id dispatch-freshness-gate (weight 0.95, width-3 batch
// 2026-07-13 postmortem): ~3 of 8 failed lane-slots were doomed at dispatch —
// a task dispatched after it shipped, a task re-picked after landing
// (consumption raced), and a task dispatched 3x against an unmet dependency.
// The gate re-resolves every spec's scope ids against current inbox/consumed
// state + deps immediately before lane launch: stale ids are skipped with a
// logged reason, freed slots are refilled from the pending backlog, and an
// honest empty-scope build after the gate verdicts SKIPPED, never FAIL.
//
// AC map (1:1), from the inbox item's acceptance[] list:
//
//	AC1 skip consumed task + refill slot        → C767_001
//	AC2 skip deps-unmet task with named reason  → C767_002
//	AC3 empty-scope build after gate → SKIPPED  → C767_003
//	AC4 go test -race PASS                      → every predicate shells the
//	    unit contract under -race (apicover runs in the repo-wide gate);
//	    C767_004 pins the adversarial negative/edge axes (anti-no-op).
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit contract in internal/fleet, which EXERCISES FreshenSpecs /
// ClassifyEmptyScopeBuild through injected probe/refill fakes — behavioral
// via subprocess, no source-grep predicates (cycle-85 rule). The `-v` +
// "--- PASS:" guard rejects a rename/no-tests-matched silent green. The unit
// contract embeds the adversarial axes: negative (all-fresh wave untouched;
// real FAIL never masked; empty scope never PASSes), edge (empty backlog
// leaves the slot unfilled; partially-stale merged scope filters ids without
// burning the slot), semantic (skip-consumed vs skip-deps-unmet vs verdict
// classification are separate behaviors).
package cycle767

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const fleetPkg = "github.com/mickeyyaya/evolve-loop/go/internal/fleet"

// runGoTest executes the named unit test under -race and requires an explicit
// verbose PASS marker so the predicate fails on: compile failure, test
// failure, a race report, a missing package, OR the test not existing
// (rename gaming).
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

// AC1: a task consumed/shipped between planning and launch is skipped with a
// logged reason and its slot is refilled from the pending backlog (exclude
// set covers every id the wave already owns, so a refill can't duplicate).
func TestC767_001_SkipsConsumedTaskAndRefillsSlot(t *testing.T) {
	runGoTest(t, fleetPkg, "TestWaveDispatch_SkipsConsumedTaskAndRefillsSlot")
}

// AC2: a task with an unmet declared dependency is skipped with a reason
// NAMING the blocking dep; an exhausted backlog leaves the slot empty
// (a shorter wave, never a doomed lane).
func TestC767_002_SkipsDepsUnmetTaskWithReason(t *testing.T) {
	runGoTest(t, fleetPkg, "TestWaveDispatch_SkipsDepsUnmetTaskWithReason")
}

// AC3: an honest empty-scope build after the freshness gate verdicts SKIPPED
// — never FAIL (never punish honesty) and never PASS (no work is not work);
// includes the negative rows (no-gate FAIL preserved, real FAIL never masked).
func TestC767_003_EmptyScopeAfterGateVerdictsSkippedNotFail(t *testing.T) {
	runGoTest(t, fleetPkg, "TestBuildEmptyScope_AfterFreshnessGate_VerdictSkippedNotFail")
}

// AC1/AC2 adversarial axes (anti-no-op): an all-fresh wave passes through
// untouched (no skips, no refills, no log noise, order preserved), and a
// merged spec with one consumed id keeps its slot with the scope filtered —
// a gate that rewrites healthy waves, or burns partially-live slots, fails here.
func TestC767_004_GateNegativeAndEdgeAxes(t *testing.T) {
	runGoTest(t, fleetPkg, "TestWaveDispatch_AllFresh_NoSkipNoRefill")
	runGoTest(t, fleetPkg, "TestWaveDispatch_PartialStaleScope_FiltersIdKeepsSpec")
}
