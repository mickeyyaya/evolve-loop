//go:build acs

// Package cycle1549 materialises the acceptance criteria for this lane's single
// fleet-scoped task, `premise-challenge-learning-real-path`
// (triage-report.md ## top_n).
//
// What this cycle is. Not new production behavior: a COMPOSED-PATH PROOF. #479
// made a judgment phase's FAIL verdict leave a carryover lesson without a halt
// vector; #481 made a well-formed verdict sentinel able to produce that FAIL
// instead of being structurally forced to PASS. Both are unit-proven in
// isolation — judgment_lesson_test.go injects a hand-built PhaseResponse, and
// the specrunner sentinel tests stop at classification. Nothing drives ONE real
// sentinel through parse → verdict → recordAndBranch → persisted state →
// next-cycle planner context. That seam can be cut while every existing test
// stays green, which is precisely the regression this cycle must make loud.
//
// Predicate strategy. Every predicate here is BEHAVIORAL: it builds and runs the
// real `internal/core` test binary as a subprocess against a named subtest and
// asserts on the emitted `--- PASS:` line. Exit code alone is deliberately NOT
// load-bearing — `go test -run` over a pattern that matches nothing exits 0 with
// "no tests to run", so an exit-code-only predicate would go GREEN on an absent
// test (the cycle-131/137 vacuity trap). The PASS-line assertion is what keeps
// these RED today and what makes a rename fail loudly instead of silently.
//
// SCOPE NARROWING, compiler-proven (the cycle-644 reachability rule). The task's
// AC says the FAIL must come from "real sentinel parsing", and the obvious
// reading is "call specrunner.EvaluateClassify". That is IMPOSSIBLE and no
// predicate here may pin it: internal/phases/specrunner imports internal/core,
// so a `package core` test importing specrunner is an import cycle. Probed:
//
//	imports .../internal/phases/specrunner from zz_probe_import_test.go
//	imports .../internal/core from specrunner.go: import cycle not allowed in test
//
// So the composed path is pinned one layer down, at the parser BOTH layers
// already share: phasecontract.ParseVerdictSentinelFull — the exact function
// specrunner's applySentinelStage calls, and a package internal/core already
// imports in production (build_removal_check.go, cyclerun_remediate.go). That is
// the single-source reading, not a second grammar: a drift in the sentinel
// vocabulary breaks this test and the classifier together, which is the property
// the AC is actually after.
//
// RED baseline (this worktree, main-based). 001-004 fail because
// TestJudgmentLessonFullPath_PremiseChallengeSentinelFAILTeachesWithoutHalting
// and TestJudgmentLessonFullPath_NoLessonWithoutAWellFormedSentinelFAIL do not
// exist in go/internal/core/judgment_lesson_test.go — `go test` exits 0 and
// prints no PASS line, so each predicate reports the vacuity explicitly.
//
// Adversarial diversity. NEGATIVE — 004 is the anti-no-op predicate: a stated
// PASS, a malformed sentinel, and an absent sentinel must ALL leave zero
// lessons, so an implementation that files a lesson unconditionally (the
// cheapest way to pass 001-003) fails here. EDGE — 004's malformed and absent
// rows are the fail-open boundary; 003 pins the FailedAt array UNCHANGED, not
// merely "small". SEMANTIC — parse-produces-FAIL, persistence-reaches-planner,
// non-halt, and fail-open-negatives are four distinct behaviors, not one
// restated.
package cycle1549

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

	// fullPathTest is the composed-path test named by scout-report.md's
	// verifiableBy. Its subtests carry ACs 1-3, one each.
	fullPathTest = "TestJudgmentLessonFullPath_PremiseChallengeSentinelFAILTeachesWithoutHalting"
	// negativeTest carries AC4 — the three fail-open rows that must leave no
	// lesson at all.
	negativeTest = "TestJudgmentLessonFullPath_NoLessonWithoutAWellFormedSentinelFAIL"
)

// judgmentLessonTestFile is the ONE file the task is allowed to touch
// (triage-report.md files={...}); the auxiliary source assertions in 001 read it.
func judgmentLessonTestFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go", "internal", "core", "judgment_lesson_test.go")
}

// runCoreSubtest runs ONE named subtest of the core package as a subprocess and
// requires an explicit `--- PASS: Parent/Sub` line.
//
// Scoped deliberately: a single named package narrowed by -run, never a `/...`
// sweep and never the whole internal/core suite (the flaky-shape suite-scope
// class — internal/core is a known 40s+ suite under fleet contention, and the
// -run narrowing is the documented remedy). No wall-clock bound, no literal PID,
// no bare `git`: nothing here can flake on load.
func runCoreSubtest(t *testing.T, parent, sub string) {
	t.Helper()
	pattern := "^" + parent + "$/^" + sub + "$"
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", pattern, corePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %q %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pattern, corePkg, code, err, stdout, stderr)
	}
	// The vacuity guard, and the reason exit code is not load-bearing: an
	// absent or renamed (sub)test makes `go test -run` exit 0 having run
	// nothing at all.
	if !strings.Contains(stdout, "--- PASS: "+parent+"/"+sub) {
		t.Errorf("subtest %s/%s did not report PASS — it is absent, renamed, skipped, or was filtered out "+
			"(exit 0 with no test run is NOT proof of anything).\nstdout:\n%s", parent, sub, stdout)
	}
}

// AC1 — "Real sentinel parsing produces the premise-challenge FAIL used by the
// test; no hand-built FAIL response substitutes for it."
//
// Load-bearing: the subtest runs and passes. Auxiliary (never the sole
// evidence): the composed-path test must reach the production sentinel parser
// AND build its fixture artifact with the production RENDERER, so neither the
// FAIL nor the artifact carrying it is a hand-typed string. CountInGoFunc errors
// loudly on a renamed function rather than satisfying a ==0 assertion silently.
func TestC1549_001_real_sentinel_parse_produces_the_fail(t *testing.T) {
	runCoreSubtest(t, fullPathTest, "real_sentinel_parse_produces_the_FAIL")

	file := judgmentLessonTestFile(t)
	parsed, err := acsassert.CountInGoFunc(file, fullPathTest, "ParseVerdictSentinel")
	if err != nil {
		t.Fatalf("CountInGoFunc(%s, %s): %v", file, fullPathTest, err)
	}
	if parsed < 1 {
		t.Errorf("%s never calls phasecontract.ParseVerdictSentinel* — the FAIL must be PRODUCED by the "+
			"production parser, not asserted about a hand-built PhaseResponse; that is the whole "+
			"composition this cycle exists to prove", fullPathTest)
	}
	rendered, err := acsassert.CountInGoFunc(file, fullPathTest, "RenderVerdictSentinel")
	if err != nil {
		t.Fatalf("CountInGoFunc(%s, %s): %v", file, fullPathTest, err)
	}
	if rendered < 1 {
		t.Errorf("%s never calls phasecontract.RenderVerdictSentinel* — a hand-typed sentinel string is a "+
			"SECOND grammar that drifts away from the one the classifier reads; render the fixture with "+
			"the production renderer so producer and parser stay in lockstep", fullPathTest)
	}
}

// AC2 — "Persisted carryover includes the exact objection and is visible through
// the next-cycle planner context."
//
// Distinct behavior from AC1: parsing correctly and persisting correctly are
// separately cuttable. The subtest must cross the storage boundary (ReadState)
// and render the planner context, not read cr.state in memory.
func TestC1549_002_objection_reaches_next_cycle_planner_context(t *testing.T) {
	runCoreSubtest(t, fullPathTest, "objection_reaches_next_cycle_planner_context")
}

// AC3 — "FailedAt is unchanged and continuation remains non-halting."
//
// The half that blocked the original fix: teaching must not import a halt
// vector. state.FailedAt feeds failureadapter.Decide (sameClassStreak /
// tailInfraTransientStreak), so a phase whose entire job is to object could halt
// the batch BY objecting. The subtest must pin the array unchanged AND the
// returned loopAction non-abort with a nil error.
func TestC1549_003_failed_at_unchanged_and_continuation_non_halting(t *testing.T) {
	runCoreSubtest(t, fullPathTest, "failed_at_unchanged_and_continuation_non_halting")
}

// AC4 — "PASS, malformed, and absent-sentinel cases leave no lesson."
//
// The NEGATIVE predicate, and the strongest anti-no-op signal in this file: an
// implementation that files a lesson unconditionally passes 001-003 and fails
// here. Each row is required by name so a suite that quietly drops the malformed
// or absent case cannot go green on the stated-PASS row alone.
func TestC1549_004_no_lesson_for_pass_malformed_or_absent_sentinel(t *testing.T) {
	for _, sub := range []string{"stated_PASS", "malformed_sentinel", "absent_sentinel"} {
		runCoreSubtest(t, negativeTest, sub)
	}
}
