//go:build acs

// Package cycle1130 materialises the cycle-1130 acceptance criteria for the
// single fleet-scoped task `surface-verdict-conflict-in-audit-classify` (inbox
// item `verdict-coherence-auditor-vs-egps`, weight 0.92, kind
// pipeline-integrity, 4th recurrence of the cycle-87 / cycle-352 / cycle-456
// family; live cases 1107 / 1116 / 1117).
//
// STATE AT HEAD — read this before treating a green run as a no-op. HEAD is
// 27059076, the ADR-0076 continuation-on-fail salvage snapshot, which carries a
// PRIOR attempt's implementation of this exact task: hooks.Classify already
// captures the auditor's narrative verdict and emits an error-severity
// `verdict-conflict:` record across all ten override paths, and the cycle1127
// predicates for the same family are green. The task's own predicates are
// therefore PRE-EXISTING GREEN at HEAD by inheritance, not by no-op.
//
// Non-degeneracy was proven by counterfactual, not by assertion: with
// go/internal/phases/audit/audit.go reverted to the pre-salvage parent
// (bc2e3236) the producer predicate below FAILS on both EGPS branches
// ("no error-severity diagnostic carries verdict-conflict / PASS / WARN"),
// while the anti-noise predicate stays green — exactly the split a correct
// guard should show. Evidence is pasted in the cycle-1130 test-report.md.
//
// What cycle 1130 adds over the inherited cycle1124/cycle1127 suites: those pin
// that a conflict record EXISTS, is verbatim, is per-gate distinguishable, and
// is silent when coherent. None of them pin the scout report's actual
// verifiableBy — that ONE Classify call hands the operator BOTH halves of the
// forensic pair (the auditor's declared verdict AND the gate's red_count /
// normalized red identity), both at Severity=="error" so both ride
// errorSeverityMessages → AuditFailReasons → <phase>-fail-reason.json → the
// dossier's SubstantiveError. A refactor that keeps the record but drops the
// gate facts beside it — or demotes either to warning — leaves the operator
// with the same half-picture cycles 1107/1116/1117 already had, and is caught
// here.
//
// Predicate strategy — BEHAVIORAL, never a source-grep (the cycle-85
// degenerate-predicate ban). `hooks` and `Classify` are unexported, so each
// predicate runs the REAL in-package test binary as a subprocess against THIS
// worktree and asserts on its exit code. No predicate here asserts that a
// source file contains a string.
//
//   - 001 producer (AC-1): both halves of the pair arrive together, at error
//     severity, on the red_count>0 and ship_eligible=false branches.
//   - 002 anti-no-op (AC-2/AC-3): the gate's evidence survives on the COHERENT
//     path where no conflict record is emitted, and the clean-PASS path emits
//     neither half. An implementation that only ever emits facts alongside a
//     conflict record greens 001 and fails here.
//   - 003 no-regression under -race (AC-4): the AC names `-race` explicitly,
//     and the inherited cycle1127 predicates run without it — this is the only
//     predicate in the family that would catch a data race introduced in the
//     conflict path.
package cycle1130

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const auditPkg = "./internal/phases/audit/"

// goDir is the worktree's go module root — predicates read the CYCLE's source,
// not main's (worktree isolation; acsassert.RepoRoot resolves the worktree).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// runGoTest runs `go test -C <worktree>/go [-race] -run ^(pattern)$ <pkg>` and
// reports whether it exited 0. A compile failure in the target package (a
// legitimate RED signal) surfaces as a non-zero exit, NOT as a launch failure:
// SubprocessOutput returns a non-nil err for any non-zero exit, so only
// code < 0 is a genuine "could not launch" (cycle-574 lesson).
func runGoTest(t *testing.T, pkg, pattern string, race bool) (ok bool, out string) {
	t.Helper()
	args := []string{"test", "-C", goDir(t), "-count=1"}
	if race {
		args = append(args, "-race")
	}
	args = append(args, "-run", "^("+pattern+")$", pkg)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", args...)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to LAUNCH for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, tail(out, 30))
	}
	return code == 0, out
}

// tail returns the last n lines — diagnostics stay readable in the verdict.
func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// TestC1130_001_ConflictAndGateFactsReachTheOperatorTogether — AC-1, the
// producer half. On both EGPS override branches a single Classify call must
// return, at error severity, the auditor's own verdict AND the gate's identity
// facts. Either half alone is the 1107/1116/1117 shape: an operator holding
// only "red_count=1" cannot tell a genuine regression from a poisoned
// predicate the auditor itself read as clean.
func TestC1130_001_ConflictAndGateFactsReachTheOperatorTogether(t *testing.T) {
	ok, out := runGoTest(t, auditPkg,
		"TestVerdictConflict_EGPSRed_NarrativeAndGateFactsArriveTogether|"+
			"TestVerdictConflict_ShipEligible_NarrativeAndGateFactsArriveTogether", false)
	if !ok {
		t.Errorf("RED: an EGPS override no longer hands the operator BOTH the auditor's declared "+
			"verdict and the gate's red_count/red-identity evidence at error severity — half the "+
			"forensic pair is missing from the dossier chain:\n%s", tail(out, 40))
	}
}

// TestC1130_002_GateEvidenceSurvivesWhereThereIsNoConflict — AC-2/AC-3, the
// anti-no-op axis. The conflict record is ADDITIVE: on the coherent path
// (auditor also said FAIL) there is no disagreement to record, but the gate's
// own evidence must still reach the dossier, and the clean-PASS path must emit
// no error diagnostics at all. An implementation that emits facts ONLY beside a
// conflict record, or that swallows the plain gate message once the conflict
// path exists, greens 001 and fails here.
func TestC1130_002_GateEvidenceSurvivesWhereThereIsNoConflict(t *testing.T) {
	ok, out := runGoTest(t, auditPkg,
		"TestVerdictConflict_GateFactsSurviveWithoutAConflict|"+
			"TestVerdictConflict_CleanPassEmitsNeitherHalf", false)
	if !ok {
		t.Errorf("RED: the gate's evidence vanished on the coherent path, a conflict was fabricated "+
			"where auditor and gate AGREED, or the clean-PASS path leaked an error diagnostic — the "+
			"record must never replace or disturb what the gate already reported:\n%s", tail(out, 40))
	}
}

// TestC1130_003_ConflictFamilyIsRaceCleanAndRegressionFree — AC-4. The AC names
// `go test ./internal/phases/audit/... -race` explicitly. The inherited
// cycle1124/cycle1127 predicates all run WITHOUT -race, so this is the only
// predicate in the family that would catch a data race introduced in the
// conflict path, and the only one that re-proves the whole conflict family
// (inherited suites included) still passes under the race detector.
func TestC1130_003_ConflictFamilyIsRaceCleanAndRegressionFree(t *testing.T) {
	ok, out := runGoTest(t, auditPkg, "TestVerdictConflict_.*|TestEGPSRedDiagnostic_.*", true)
	if !ok {
		t.Errorf("RED: the verdict-conflict family (including the inherited cycle-1124/1127 suites "+
			"and the EGPS red-identity fingerprint contract) is not green under -race — either a "+
			"regression or a data race in the conflict path:\n%s", tail(out, 40))
	}
}
