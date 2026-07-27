//go:build acs

// Package cycle1127 materialises the cycle-1127 acceptance criteria for the
// single fleet-scoped task `emit-verdict-conflict-diagnostic` (inbox item
// `verdict-coherence-auditor-vs-egps`, weight 0.92, kind pipeline-integrity,
// 4th recurrence of the cycle-87 / cycle-352 / cycle-456 family).
//
// State at HEAD (33596bb0, the cycle-1124 salvage snapshot). `hooks.Classify`
// already captures the auditor's narrative verdict and records an
// error-severity `verdict-conflict:` diagnostic — but ONLY inside the EGPS
// override block (acs-verdict.json unreadable / red_count>0 /
// ship_eligible=false). Five further gates in the same function still clobber
// `verdict` to core.VerdictFAIL with no record at all:
//
//	gofmt · skills-drift · go vet · acs-durable · integration-tier ·
//	apicover-enforce · apicover new-package graduation
//
// AC-1 names those gates explicitly, so the task is not done: on 7 of 10
// override paths the operator still cannot tell a genuine defect from a
// poisoned/non-hermetic gate the auditor itself read as clean — which is the
// exact forensic cost the inbox item was filed for (cycles 1107/1116/1117,
// the connected `audit-probe-tree-isolation` case).
//
// Predicate strategy — BEHAVIORAL, never a source-grep (the cycle-85
// degenerate-predicate ban). `hooks` and `Classify` are unexported, so each
// predicate runs the REAL in-package test binary as a subprocess against THIS
// worktree and asserts on its exit code. No predicate here asserts that a
// source file contains a string.
//
//   - 001 producer: every non-EGPS gate that overrides a found, non-FAIL
//     narrative leaves exactly one error-severity conflict record naming it.
//   - 002 anti-no-op: the coherent, unparseable, fail-OPEN and all-green cases
//     emit ZERO records. An unconditional append greens 001 and fails here.
//   - 003 additive-only (AC-4): the returned verdict is byte-identical to
//     today across the narrative x gate-state matrix, and the pre-existing
//     audit suites (EGPS override, red-identity fingerprint, gofmt,
//     skills-drift, CI-parity) stay green.
//   - 004 consumer wiring: an error-severity conflict record reaches
//     CycleState.AuditFailReasons → <phase>-fail-reason.json → the dossier's
//     SubstantiveError, and a warning-severity one would be silently dropped.
package cycle1127

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
// whether it exited 0. A compile failure in the target package (a legitimate
// RED signal) surfaces as a non-zero exit, NOT as a launch failure:
// SubprocessOutput returns a non-nil err for any non-zero exit, so only
// code < 0 is a genuine "could not launch" (cycle-574 lesson).
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

// TestC1127_001_EveryGateRecordsTheVerdictConflict — AC-1. The producer half,
// and the only predicate that is RED at HEAD. Each of the seven non-EGPS gates
// must record the auditor-vs-gate disagreement, carrying the narrative verdict
// verbatim (PASS vs WARN must not collapse to one message).
func TestC1127_001_EveryGateRecordsTheVerdictConflict(t *testing.T) {
	ok, out := runGoTest(t, auditPkg,
		"TestVerdictConflict_EveryNonEGPSGateRecordsTheConflict|"+
			"TestVerdictConflict_NonEGPSGate_NarrativeWARN")
	if !ok {
		t.Errorf("RED: the gofmt / skills-drift / go-vet / acs-durable / integration-tier / "+
			"apicover-enforce / apicover-newpkg gates still overwrite the auditor's narrative "+
			"verdict with FAIL without recording the disagreement — 7 of 10 override paths leave "+
			"the operator unable to tell a real defect from a poisoned gate:\n%s", tail(out, 40))
	}
}

// TestC1127_002_NoConflictRecordWithoutAnOverride — AC-2/AC-3, the anti-no-op
// axis. A blanket "append a conflict record next to every gate diagnostic"
// implementation greens 001 and FAILS here, because the fail-OPEN paths emit a
// gate diagnostic WITHOUT overriding the verdict. Also pins the one-record-per-
// Classify rule: four simultaneously-firing gates are one conflict, not four.
func TestC1127_002_NoConflictRecordWithoutAnOverride(t *testing.T) {
	ok, out := runGoTest(t, auditPkg,
		"TestVerdictConflict_NonEGPSGate_NoNoiseWhenNarrativeFAIL|"+
			"TestVerdictConflict_NonEGPSGate_NoNoiseWhenNarrativeUnparseable|"+
			"TestVerdictConflict_GateCouldNotRun_NoConflict|"+
			"TestVerdictConflict_AllGatesGreen_NoConflict|"+
			"TestVerdictConflict_MultipleGatesStillOneRecord|"+
			"TestVerdictConflict_NoNoiseWhenNarrativeAlreadyFAIL|"+
			"TestVerdictConflict_NoNoiseWhenNarrativeUnparseable|"+
			"TestVerdictConflict_NoNoiseWhenGateGreen|"+
			"TestVerdictConflict_SingleRecordPerClassify")
	if !ok {
		t.Errorf("RED: a conflict record is emitted where the auditor and the machine did NOT "+
			"disagree (coherent FAIL, unparseable narrative, fail-OPEN gate, green gate), or one "+
			"Classify call produced more than one record — fabricated/duplicated conflicts dilute "+
			"the signal the record exists to carry:\n%s", tail(out, 40))
	}
}

// TestC1127_003_RecordIsAdditiveAndSuitesStayGreen — AC-4. The record must
// never change what ships. The tempting wrong turn on a diagnosability task is
// to soften a gate so the conflict stops appearing; the verdict matrix plus the
// pre-existing gate suites make that fail loudly.
func TestC1127_003_RecordIsAdditiveAndSuitesStayGreen(t *testing.T) {
	if ok, out := runGoTest(t, auditPkg, "TestVerdictConflict_VerdictUnchangedAcrossGateMatrix"); !ok {
		t.Errorf("RED: the returned verdict changed for some (narrative x gate-state) combination — "+
			"the conflict record must be ADDITIVE, never a softening of a gate:\n%s", tail(out, 40))
	}
	if ok, out := runGoTest(t, auditPkg,
		"TestRun_Gofmt.*|TestRun_SkillsDrift.*|TestEGPSRedDiagnostic_.*|TestClassify.*|TestAudit.*|TestCIParity.*|TestApplyCIGate.*"); !ok {
		t.Errorf("RED: a pre-existing audit-phase gate suite regressed (EGPS override, gofmt, "+
			"skills-drift, CI-parity or the red-identity fingerprint contract was disturbed):\n%s", tail(out, 40))
	}
}

// TestC1127_004_ConflictRecordReachesDossierPlumbing — the wiring proof. A
// record nothing reads is inert. Error severity IS the wiring:
// errorSeverityMessages (core/system_failure.go) keys off Severity=="error",
// so the record rides the existing AuditFailReasons → fail-reason.json →
// dossier SubstantiveError chain with no new plumbing; a warning-severity
// record would be silently dropped there.
func TestC1127_004_ConflictRecordReachesDossierPlumbing(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestVerdictConflict_ErrorDiagnosticReachesAuditFailReasons|"+
			"TestVerdictConflict_WarningSeverityWouldBeDropped")
	if !ok {
		t.Errorf("RED: the verdict-conflict record does not flow through the existing "+
			"errorSeverityMessages → AuditFailReasons → dossier chain — an unread record is "+
			"an unfixed defect:\n%s", tail(out, 40))
	}
}
