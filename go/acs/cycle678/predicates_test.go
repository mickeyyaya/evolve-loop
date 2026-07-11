//go:build acs

// Package cycle678 materialises the acceptance criteria for cycle 678's
// triage-committed task, inbox `new-package-graduation-buildentry-gate`
// (weight 0.92, RETRY — 3rd recurrence: cycles 575/587/652).
//
// CLOSURE-VERIFICATION CYCLE: the implementation itself landed in cycle 675
// (commit 1370807e — buildGraduationCheck + abort wiring at the post-build
// seam, audit-side gate registration pinned; audit PASS 0.92). The inbox item
// re-triaged as RETRY only because its id never matched cycle-675's
// differently-named slugs (`build-entry-graduation-guard[-audit-regression]`),
// so it was never promoted out of .evolve/inbox/. This cycle closes the item
// under its OWN id: predicates re-pin every acceptance criterion against HEAD
// so a regression to the landed guard fails here, and C678_005 adds the one
// check cycle 675 did not predicate — the inbox fix seam (2): the repo-wide
// apicover COMPLETENESS predicate is inside the cycle audit's repo-wide gate
// set and GREEN at HEAD.
//
// Predicate strategy: behavioural-via-subprocess (cycle-549…675 precedent) —
// each predicate shells `go test -run` over unit tests that EXERCISE the SUT
// (buildGraduationCheck over real git worktree fixtures; recordAndBranch
// (PhaseBuild); the production-constructed audit phase end-to-end); none is
// source-grep. All are verify-only GREEN by design (regression pins for a
// landed fix), honestly declared — the RED-first arm of this item was run and
// satisfied in cycle 675.
package cycle678

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg           = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	auditPkg          = "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
	completenessPkg   = "github.com/mickeyyaya/evolve-loop/go/acs/regression/apicover"
	auditGateTestGlob = "github.com/mickeyyaya/evolve-loop/go/acs/regression/..."
)

// runGo shells `go <args...>` and reports whether it exited cleanly plus the
// combined output. -count=1 (passed by callers of `go test`) defeats the test
// cache so a predicate always exercises current source. code<0 is a genuine
// launch failure (binary missing / killed by signal), never a test verdict —
// that fails loudly rather than being misread as a RED result.
func runGo(t *testing.T, args ...string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", args...)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("could not launch go %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return code == 0, out
}

func runGoTest(t *testing.T, pkg, pattern string, extraArgs ...string) (ok bool, out string) {
	t.Helper()
	args := append([]string{"test", "-run", pattern, "-count=1"}, extraArgs...)
	return runGo(t, append(args, pkg)...)
}

// TestC678_001_BuildGraduationCheckTable — AC1 detection: the table-driven
// contract for buildGraduationCheck (new-ungraduated fails with a reason
// naming the package + .apicover-enforce; enrolled/cmd/missing-enforce/
// non-Go changes pass). Exercises the SUT over real git worktree fixtures.
func TestC678_001_BuildGraduationCheckTable(t *testing.T) {
	if ok, out := runGoTest(t, corePkg, "^TestBuildGraduationCheck$"); !ok {
		t.Errorf("build-entry graduation check contract not green:\n%s", out)
	}
}

// TestC678_002_BuildPhaseGraduationAbortWiring — AC1 wiring: at the
// post-build seam (recordAndBranch(PhaseBuild)) an ungraduated new package
// FAILS the phase with an explicit abort_reason on the recorded outcome, and
// an enrolled package proceeds loopNext — abort-capable, unlike the WARN-only
// buildSelfCheck.
func TestC678_002_BuildPhaseGraduationAbortWiring(t *testing.T) {
	if ok, out := runGoTest(t, corePkg,
		"^(TestRecordAndBranch_BuildGraduationGuardAborts|TestRecordAndBranch_BuildGraduationGuardEnrolledProceeds)$"); !ok {
		t.Errorf("build-phase graduation abort wiring not green:\n%s", out)
	}
}

// TestC678_003_AuditGraduationGateRegistered — AC2 (in-process half): the
// PRODUCTION audit constructor (NewDefaultWithStageCompact) still registers
// the new-package graduation gate — an ungraduated package FAILs the audit
// end-to-end, an enrolled one PASSes.
func TestC678_003_AuditGraduationGateRegistered(t *testing.T) {
	if ok, out := runGoTest(t, auditPkg, "^TestNewDefaultWithStageCompact_GraduationGateRegistered$"); !ok {
		t.Errorf("audit-side graduation gate registration regression not green:\n%s", out)
	}
}

// TestC678_004_NoFalsePositiveOnDeleteRename — AC3 (the negative axis): a
// package deleted this cycle (enforce entry removed in the same diff), a
// rename whose destination is enrolled, and a new package self-graduating in
// the same diff must all pass the guard.
func TestC678_004_NoFalsePositiveOnDeleteRename(t *testing.T) {
	if ok, out := runGoTest(t, corePkg,
		"^TestBuildGraduationCheck$/^(deleted-package-not-flagged|renamed-package-reenrolled-passes|enrolled-same-diff-passes)$"); !ok {
		t.Errorf("graduation guard false-positive arms not green:\n%s", out)
	}
}

// TestC678_005_RepoWideCompletenessPredicateInAuditGateSet — AC2 (repo-wide
// half, the inbox fix seam (2) verbatim): the repo-wide apicover COMPLETENESS
// predicate (TestApicoverEnforce_CoversEveryInternalPackage, the check that
// every ./internal/... package appears in go/.apicover-enforce) is (a) inside
// the package set the cycle audit's acs-durable CI-parity gate runs
// (acsDurableCheckDefault shells `go test -tags acs ./acs/regression/...`,
// which recursively includes acs/regression/apicover), and (b) GREEN at HEAD
// — i.e. apicover completeness is clean, the "apicover clean" arm of AC4.
// If the predicate package ever moves out of the acs-durable glob, arm (a)
// fails and the audit's repo-wide gate set silently loses the completeness
// check — exactly the strict-subset disease this inbox item names.
func TestC678_005_RepoWideCompletenessPredicateInAuditGateSet(t *testing.T) {
	ok, out := runGo(t, "list", "-tags", "acs", auditGateTestGlob)
	if !ok {
		t.Fatalf("go list over the acs-durable gate glob failed:\n%s", out)
	}
	if !strings.Contains(out, completenessPkg) {
		t.Errorf("completeness predicate package %s is NOT inside the acs-durable audit gate glob %s — the audit's repo-wide gate set no longer runs TestApicoverEnforce_CoversEveryInternalPackage.\ngo list output:\n%s",
			completenessPkg, auditGateTestGlob, out)
	}
	if ok, tout := runGoTest(t, completenessPkg,
		"^TestApicoverEnforce_CoversEveryInternalPackage$", "-tags", "acs"); !ok {
		t.Errorf("repo-wide apicover completeness predicate not green at HEAD:\n%s", tout)
	}
}
