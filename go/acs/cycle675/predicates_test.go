//go:build acs

// Package cycle675 materialises the acceptance criteria for cycle 675's
// triage-committed tasks (inbox new-package-graduation-buildentry-gate RETRY,
// weight 0.92 — 3rd recurrence, cycles 575/587/652):
//
//   - Task 1 build-entry-graduation-guard: a deterministic build-phase gate
//     that FAILS the phase (explicit abort_reason) when a changed
//     go/internal/<pkg> is new this cycle and absent from go/.apicover-enforce.
//     AC1 (C675_001, C675_002): table-driven check + recordAndBranch wiring.
//     AC3 (C675_004): no false positive on delete/rename/self-graduating diffs.
//   - Task 2 build-entry-graduation-guard-audit-regression: the audit-side
//     gate (apicoverNewPackageGraduationDefault, wired 2026-07-07) stays
//     registered through the PRODUCTION constructor. AC2 (C675_003) —
//     verify-only: pre-existing GREEN by design; it regression-proofs the
//     already-landed half so the two seams can never again silently diverge.
//
// Predicate strategy: behavioural-via-subprocess (cycle-549…672 precedent) —
// each predicate shells `go test -run` over unit tests that EXERCISE the SUT
// (buildGraduationCheck over real git worktrees; recordAndBranch(PhaseBuild);
// the production-constructed audit phase end-to-end); none is source-grep.
// RED now: internal/core fails to compile (buildGraduationCheck absent).
// GREEN once Builder lands the guard. The Acceptance-Criteria-Summary line
// "go test -race on touched packages PASS" is dispositioned manual+checklist
// in test-report.md (the cycle audit's repo-wide CI-parity gates own it), not
// predicated here.
package cycle675

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg  = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	auditPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
)

// runGoTest shells `go test -run '<pattern>' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the
// test cache so the predicate always exercises current source. code<0 is a
// genuine launch failure (binary missing / killed by signal), never a test
// verdict — that fails loudly rather than being misread as a RED result.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("could not launch go test %s: %v\n%s", pkg, err, out)
	}
	return code == 0, out
}

// TestC675_001_BuildGraduationCheckTable — AC1 detection: the table-driven
// contract for buildGraduationCheck (new-ungraduated fails with a reason
// naming the package + .apicover-enforce; enrolled/cmd/missing-enforce/
// non-Go changes pass). Exercises the SUT over real git worktree fixtures.
func TestC675_001_BuildGraduationCheckTable(t *testing.T) {
	if ok, out := runGoTest(t, corePkg, "^TestBuildGraduationCheck$"); !ok {
		t.Errorf("build-entry graduation check contract not green:\n%s", out)
	}
}

// TestC675_002_BuildPhaseGraduationAbortWiring — AC1 wiring: at the post-build
// seam (recordAndBranch(PhaseBuild)) an ungraduated new package FAILS the
// phase with an explicit abort_reason on the recorded outcome, and an
// enrolled package proceeds loopNext — the guard is abort-capable, unlike the
// WARN-only unit-test self-check.
func TestC675_002_BuildPhaseGraduationAbortWiring(t *testing.T) {
	if ok, out := runGoTest(t, corePkg,
		"^(TestRecordAndBranch_BuildGraduationGuardAborts|TestRecordAndBranch_BuildGraduationGuardEnrolledProceeds)$"); !ok {
		t.Errorf("build-phase graduation abort wiring not green:\n%s", out)
	}
}

// TestC675_003_AuditGraduationGateRegistered — AC2 (verify-only regression):
// the PRODUCTION audit constructor (NewDefaultWithStageCompact) still
// registers the new-package graduation gate — an ungraduated package FAILs
// the audit end-to-end, an enrolled one PASSes. Guards the already-landed
// audit half against silently dropping out (the cycle-652 divergence class).
func TestC675_003_AuditGraduationGateRegistered(t *testing.T) {
	if ok, out := runGoTest(t, auditPkg, "^TestNewDefaultWithStageCompact_GraduationGateRegistered$"); !ok {
		t.Errorf("audit-side graduation gate registration regression not green:\n%s", out)
	}
}

// TestC675_004_NoFalsePositiveOnDeleteRename — AC3 (the negative axis): a
// package deleted this cycle (enforce entry removed in the same diff), a
// rename whose destination is enrolled, and a new package self-graduating in
// the same diff must all pass the guard. A gate that false-positives on
// hygiene diffs would make graduation maintenance un-shippable.
func TestC675_004_NoFalsePositiveOnDeleteRename(t *testing.T) {
	if ok, out := runGoTest(t, corePkg,
		"^TestBuildGraduationCheck$/^(deleted-package-not-flagged|renamed-package-reenrolled-passes|enrolled-same-diff-passes)$"); !ok {
		t.Errorf("graduation guard false-positive arms not green:\n%s", out)
	}
}
