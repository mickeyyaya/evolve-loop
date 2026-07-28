//go:build acs

// Package cycle1155 materialises the cycle-1155 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//	replan-rejections-telemetry — wire router.ValidatePlan into the post-scout
//	re-plan path (internal/core/cyclerun_replan.go) so unknown-phase rejections in
//	a RE-PLAN are recorded, and fix recordPlanRejections
//	(internal/core/decision_branch.go) so a second call in the same cycle
//	ACCUMULATES per plan-kind instead of overwriting the upfront record.
//
// Why this is the cycle's bar. The enforcement half already shipped: the
// integrity-floor clamp drops unknown-phase entries on BOTH the upfront and the
// re-plan path (router/floor.go dropUnknownPhases). The telemetry half did not —
// router.ValidatePlan has exactly one call site (core/cyclerun.go, upfront plan),
// so an advisor-hallucinated phase in a re-plan is dropped SILENTLY with zero
// forensic trail, which is the exact failure dropUnknownPhases' doc comment cites
// (cycles 1151, 1152). And recordPlanRejections writes advisor-rejections.json
// unconditionally, so naively adding the second call site would DESTROY the
// upfront record — the accumulation half is not optional polish, it is what makes
// the wiring safe.
//
// Predicate strategy — every predicate EXERCISES the system (cycle-85
// degenerate-predicate ban): each shells `go test` on a behavioural test in
// internal/core that drives the real cycleRun.postScoutReplan() against a real
// workspace and reads the real emitted telemetry artifacts. None of them greps
// production source for a magic string, so adding the string `ValidatePlan` to
// cyclerun_replan.go without a working record cannot pass any of them.
//
// Asserting on the `--- PASS: <name>` line rather than the exit code is load
// bearing: `go test -run` on a pattern matching NOTHING exits 0 with "no tests to
// run", so a deleted or renamed binding test would otherwise false-GREEN.
package cycle1155

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg   = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	routerPkg = "github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// assertDefaultSuiteTestsPass shells `go test -run '^(names)$' -v -count=1 pkg`
// in the DEFAULT build suite and requires EVERY name to have printed a
// `--- PASS: <name>` line. -count=1 defeats the test cache so the predicate
// always exercises current source.
func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-v", "-count=1", pkg)
	if code == -1 {
		// The subprocess never launched (toolchain/module resolution failure) — a
		// harness error, not a test verdict.
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("binding test %s did NOT pass in %s (missing, renamed, or failing). exit=%d\n"+
				"combined go-test output:\n%s", name, pkg, code, out)
		}
	}
}

// TestC1155_001_ReplanUnknownPhaseRecordsRejection — AC1 (crux). A post-scout
// re-plan naming a phase outside the known-phase set must leave a forensic
// record: an "unknown-phase" rejection for that phase, recoverable from the
// workspace and attributable to the re-plan. The binding test drives the real
// cycleRun.postScoutReplan() with a planner that hallucinates a phase and then
// reads the emitted advisor-rejections*.json.
//
// Cheapest gaming fake and why it is caught: validating the CLAMPED re-plan
// instead of the RAW one records nothing at all (the clamp already deleted the
// phantom entry), so the binding test still fails — ValidatePlan must run
// pre-clamp, exactly as it does on the upfront path.
func TestC1155_001_ReplanUnknownPhaseRecordsRejection(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestReplan_UnknownPhase_RecordsRejection")
}

// TestC1155_002_ReplanCleanNoSpuriousAndUpfrontSurvives — AC2 (negative +
// no-data-loss). An all-known re-plan must record NO unknown-phase rejection,
// must still leave proof that validation RAN on the re-plan (recordPlanRejections'
// own contract: "[]" = validated-clean, distinct from "validation never ran"),
// and must NOT destroy the upfront plan's already-written record.
//
// Cheapest gaming fake and why it is caught: recording the re-plan by rewriting
// advisor-rejections.json (today's unconditional write) satisfies the "proof it
// ran" half but erases the seeded upfront record, so the binding test fails on
// the survival assertion. Suppressing the write entirely fails the proof-it-ran
// assertion. Only accumulation passes both.
func TestC1155_002_ReplanCleanNoSpuriousAndUpfrontSurvives(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestReplan_KnownPhasesOnly_NoSpuriousRejection")
}

// TestC1155_003_MultipleReplansAllRejectionsRecorded — AC3 (accumulation under
// depth > 1). RePlanMaxDepth permits more than one re-plan per cycle; each
// re-plan's rejections must stay recoverable. The binding test runs two re-plans
// naming two DIFFERENT unknown phases and requires both to survive.
//
// Cheapest gaming fake and why it is caught: an "upfront + latest re-plan"
// two-slot scheme still loses the first re-plan's record, so the first phantom is
// unrecoverable and the binding test fails.
func TestC1155_003_MultipleReplansAllRejectionsRecorded(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg, "TestReplan_MultipleReplans_AllRejectionsRecorded")
}

// TestC1155_004_CoreAndRouterSuitesGreen — AC4 (no regression). The change
// touches a shared artifact writer (recordPlanRejections) that the upfront plan
// path and its existing telemetry tests also depend on, plus the router package
// whose ValidatePlan/floor contract is being newly consumed. Both full packages
// must stay green — a fix that satisfies AC1–AC3 by changing the upfront record's
// shape out from under decision_branch_rejections_test.go is not a fix.
func TestC1155_004_CoreAndRouterSuitesGreen(t *testing.T) {
	for _, pkg := range []string{corePkg, routerPkg} {
		stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", pkg)
		if code == -1 {
			t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
		}
		if code != 0 {
			t.Errorf("regression: %s is not green (exit=%d)\n%s", pkg, code, stdout+stderr)
		}
	}
}
