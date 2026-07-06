//go:build acs

// Package cycle552 materialises the cycle-552 acceptance criteria for this
// fleet lane's sole `## top_n` task per triage-report.md:
//
//	eliminate-sequential-fallback-min-width-lane — verify and close the
//	residual test-coverage gap on the already-landed (cycle-547) min-width
//	fleet-dispatch repair. Per triage-report.md's Rationale, the repair
//	itself (forceOneLaneDispatch / dispatchIteration) is already correctly
//	implemented and independently unit-tested; the gap is that nothing
//	exercises the RunLoop call-site wiring (cmd_loop.go's batch for-loop)
//	that connects them: the `fleetCfg.Count > 1 && waveCfg.Count <= 1` guard,
//	the one-lane launcher construction, and the WARN-vs-dispatch branching
//	that decides continue-vs-sequential-fallback. That wiring could silently
//	regress (an inverted guard, or the call site deleted in an unrelated
//	refactor) without any existing test catching it.
//
// Per triage-report.md's "Fleet scope note", the scout-proposed
// runlease-pid-aware-liveness and memo-overlay-merge-activation tasks are
// OUT OF SCOPE for this fleet-scoped lane (assigned to a different lane's
// triage pass) and are NOT predicated here (AC-Materialization Contract
// R9.3: predicates bind ONLY to triage-committed `## top_n` work).
//
// Predicate strategy (mirrors cycle547/549/550): BEHAVIORAL predicates drive
// the system under test through its in-package RED tests via subprocess
// `go test`, asserting a non-degenerate pass (requireTestsRan closes the
// cycle-85 "no tests to run" trap) — never a source grep. The in-package
// tests were authored by the TDD engineer this cycle:
//
//	cmd/evolve/cmd_loop_wave_minwidth_wiring_test.go (new minWidthRepair contract)
//
// RED today: the package fails to BUILD (minWidthRepair is undefined) — a
// subprocess `go test` exits non-zero with a compile error, which is exactly
// what every predicate below asserts against. The Builder extracts
// minWidthRepair from RunLoop's inline switch (byte-identical stderr
// messages + control flow) and wires the call site through it; it must not
// modify the tests.
package cycle552

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

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

// TestC552_001_MinWidthRepair_GuardConditionGatesEligibility (AC1: "guard
// condition" — fleetCfg.Count<=1 must never invoke the repair at all,
// leaving preflight/planFn/launcher untouched). Drives
// cmd_loop_wave_minwidth_wiring_test.go. RED today: minWidthRepair
// undefined (package cmd/evolve test build fails).
func TestC552_001_MinWidthRepair_GuardConditionGatesEligibility(t *testing.T) {
	out, code := runGoTest(t, "TestMinWidthRepair_GuardNotMetNeverInvokesLauncher", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("minWidthRepair must gate on fleetCfg.Count>1 && waveCfg.Count<=1 and never touch preflight/planFn/launcher when ineligible (exit=%d)\n%s", code, out)
	}
}

// TestC552_002_MinWidthRepair_DispatchesOneIsolatedLaneAndContinues (AC2:
// "launcher construction" — an eligible wave with a real candidate must
// dispatch exactly one isolated lane and signal the caller to continue
// rather than fall through to sequential). Drives
// cmd_loop_wave_minwidth_wiring_test.go. RED today: same build failure.
func TestC552_002_MinWidthRepair_DispatchesOneIsolatedLaneAndContinues(t *testing.T) {
	out, code := runGoTest(t, "TestMinWidthRepair_GuardMetDispatchesOneIsolatedLaneAndSignalsContinue", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("minWidthRepair must dispatch exactly one isolated lane through the injected launcher and report handled=true (exit=%d)\n%s", code, out)
	}
}

// TestC552_003_MinWidthRepair_EmptyBacklogPreservesTrueSequentialFallback
// (AC3: "WARN-vs-dispatch branching", empty-backlog edge — the case true
// sequential fallback stays reserved for). Drives
// cmd_loop_wave_minwidth_wiring_test.go. RED today: same build failure.
func TestC552_003_MinWidthRepair_EmptyBacklogPreservesTrueSequentialFallback(t *testing.T) {
	out, code := runGoTest(t, "TestMinWidthRepair_EligibleButEmptyBacklogFallsBackToSequential", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("an eligible-but-empty backlog must never invoke the launcher and must WARN the empty-backlog message (exit=%d)\n%s", code, out)
	}
}

// TestC552_004_MinWidthRepair_ForceDispatchErrorNeverSilentlySwallowed (AC4:
// error path — a preflight/plan-adapt error from forceOneLaneDispatch must
// surface in the WARN, never silently dropped). Drives
// cmd_loop_wave_minwidth_wiring_test.go. RED today: same build failure.
func TestC552_004_MinWidthRepair_ForceDispatchErrorNeverSilentlySwallowed(t *testing.T) {
	out, code := runGoTest(t, "TestMinWidthRepair_ForceDispatchErrorSurfacesAndFallsBack", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("a forceOneLaneDispatch error must surface in the WARN message and report handled=false (exit=%d)\n%s", code, out)
	}
}
