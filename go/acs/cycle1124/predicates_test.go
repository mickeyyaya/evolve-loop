//go:build acs

// Package cycle1124 materialises the cycle-1124 acceptance criteria for the
// single fleet-scoped task `emit-verdict-conflict-diagnostic` (inbox item
// `verdict-coherence-auditor-vs-egps`, weight 0.92, kind pipeline-integrity).
//
// Defect. `hooks.Classify` (go/internal/phases/audit/audit.go:131-179) extracts
// the auditor's narrative verdict and then unconditionally overwrites it with
// core.VerdictFAIL at three EGPS gate branches (acs-verdict.json unreadable,
// red_count>0, ship_eligible=false) without recording what the narrative said.
// Every downstream consumer (errorSeverityMessages → AuditFailReasons →
// <phase>-fail-reason.json → dossier SubstantiveError) therefore sees only the
// gate's own message, so an operator cannot tell a genuine defect from a
// POISONED predicate the auditor itself flagged clean (cycles 1116/1107/1117,
// the connected `audit-probe-tree-isolation` case).
//
// Predicate strategy — BEHAVIORAL, never a source-grep (the cycle-85
// degenerate-predicate ban). `hooks` and `Classify` are unexported, so each
// predicate runs the REAL in-package test binary as a subprocess against the
// worktree and asserts on its exit code. A no-op implementation cannot green
// these: 002 is the anti-noise negative axis (a blanket "always append a
// conflict diagnostic" fix fails it), and 003 pins the untouched-gate
// regression suites.
//
//   - 001 the producer half: the conflict record exists, is error-severity,
//     names the narrative verdict, and fires on all three override branches.
//   - 002 the negative/anti-no-op half: coherent narrative-FAIL, unparseable
//     narrative, and green-gate cases emit NO conflict record.
//   - 003 the consumer half: the record reaches AuditFailReasons / the forensic
//     file / the dossier's SubstantiveError with no new plumbing.
//   - 004 no regression: the pre-existing audit + core verdict suites stay green
//     (the gate's FAIL override must not have been softened).
package cycle1124

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	auditPkg = "./internal/phases/audit/"
	corePkg  = "./internal/core/"
)

// goDir is the worktree's go module root — predicates read the CYCLE's source,
// not main's (worktree isolation; acsassert.RepoRoot resolves the worktree).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// runGoTest runs `go test -C <worktree>/go -run ^(pattern)$ <pkg>` and reports
// whether it exited 0. A compile failure in the target package (the expected
// RED signal before Builder implements) surfaces as a non-zero exit, NOT as a
// launch failure: SubprocessOutput returns a non-nil err for any non-zero exit,
// so only code < 0 is a genuine "could not launch" (cycle-574 lesson).
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go",
		"test", "-C", goDir(t), "-count=1", "-run", "^("+pattern+")$", pkg)
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

// TestC1124_001_ConflictRecordEmittedOnEveryOverrideBranch — AC-1/AC-2. The
// producer half: Classify emits an error-severity `verdict-conflict:` record
// naming the auditor's narrative verdict at each of the three EGPS override
// branches, and the branches/narratives are distinguishable (the failure
// fingerprint must not collapse — batch-12 breaker lesson).
func TestC1124_001_ConflictRecordEmittedOnEveryOverrideBranch(t *testing.T) {
	ok, out := runGoTest(t, auditPkg,
		"TestVerdictConflict_RedCountBranch|TestVerdictConflict_ShipEligibleBranch|"+
			"TestVerdictConflict_ACSErrorBranch|TestVerdictConflict_BranchesAreDistinguishable|"+
			"TestVerdictConflict_NarrativeVerdictIsCarriedVerbatim|"+
			"TestVerdictConflict_SingleRecordPerClassify")
	if !ok {
		t.Errorf("RED: hooks.Classify does not record the auditor-vs-EGPS verdict conflict as an "+
			"error-severity diagnostic on all three override branches:\n%s", tail(out, 40))
	}
}

// TestC1124_002_NoConflictRecordOnCoherentCases — AC-3, the anti-no-op axis. A
// blanket "always append" implementation greens 001 and FAILS here: narrative
// already FAIL, narrative unparseable, and a green gate must each produce zero
// conflict records. This predicate is deliberately GREEN before the fix (no
// record exists yet, so no noise exists either) — it is the guard that stops
// the fix from being implemented as an unconditional append.
func TestC1124_002_NoConflictRecordOnCoherentCases(t *testing.T) {
	ok, out := runGoTest(t, auditPkg,
		"TestVerdictConflict_NoNoiseWhenNarrativeAlreadyFAIL|"+
			"TestVerdictConflict_NoNoiseWhenNarrativeUnparseable|"+
			"TestVerdictConflict_NoNoiseWhenGateGreen")
	if !ok {
		t.Errorf("RED: the conflict record is emitted on COHERENT cases (noise) or duplicated — "+
			"a record that fires when the auditor and the gate AGREE carries no signal:\n%s", tail(out, 40))
	}
}

// TestC1124_003_ConflictRecordReachesDossierPlumbing — AC-4, the wiring proof.
// The consumer half: an error-severity conflict diagnostic lands in
// CycleState.AuditFailReasons + the forensic <phase>-fail-reason.json (hence the
// dossier's SubstantiveError), and a warning-severity one would be dropped.
func TestC1124_003_ConflictRecordReachesDossierPlumbing(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestVerdictConflict_ErrorDiagnosticReachesAuditFailReasons|"+
			"TestVerdictConflict_WarningSeverityWouldBeDropped")
	if !ok {
		t.Errorf("RED: the verdict-conflict record does not flow through the existing "+
			"errorSeverityMessages → AuditFailReasons → dossier chain:\n%s", tail(out, 40))
	}
}

// TestC1124_004_ExistingVerdictSuitesStayGreen — AC-5. The record is ADDITIVE:
// the EGPS override, the red-identity fingerprint contract, the reconcile paths
// and the coherence floor must all be untouched. A "fix" that softens the
// gate's FAIL to make the conflict disappear fails here.
func TestC1124_004_ExistingVerdictSuitesStayGreen(t *testing.T) {
	if ok, out := runGoTest(t, auditPkg, "TestEGPSRedDiagnostic_.*|TestClassify.*|TestAudit.*"); !ok {
		t.Errorf("RED: pre-existing audit-phase verdict suite regressed (the EGPS override or the "+
			"red-identity fingerprint contract was disturbed):\n%s", tail(out, 40))
	}
	if ok, out := runGoTest(t, corePkg, "TestPersistFloorFailReasons.*|TestDetectVerdictIncoherence.*"); !ok {
		t.Errorf("RED: pre-existing core coherence-floor suite regressed:\n%s", tail(out, 40))
	}
}
