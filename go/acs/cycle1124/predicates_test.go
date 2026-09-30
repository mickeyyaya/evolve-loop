//go:build acs

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

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

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

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

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

func TestC1124_003_ConflictRecordReachesDossierPlumbing(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestVerdictConflict_ErrorDiagnosticReachesAuditFailReasons|"+
			"TestVerdictConflict_WarningSeverityWouldBeDropped")
	if !ok {
		t.Errorf("RED: the verdict-conflict record does not flow through the existing "+
			"cyclestate.ErrorMessages → AuditFailReasons → dossier chain:\n%s", tail(out, 40))
	}
}

func TestC1124_004_ExistingVerdictSuitesStayGreen(t *testing.T) {
	if ok, out := runGoTest(t, auditPkg, "TestEGPSRedDiagnostic_.*|TestClassify.*|TestAudit.*"); !ok {
		t.Errorf("RED: pre-existing audit-phase verdict suite regressed (the EGPS override or the "+
			"red-identity fingerprint contract was disturbed):\n%s", tail(out, 40))
	}
	if ok, out := runGoTest(t, corePkg, "TestPersistFloorFailReasons.*|TestDetectVerdictIncoherence.*"); !ok {
		t.Errorf("RED: pre-existing core coherence-floor suite regressed:\n%s", tail(out, 40))
	}
}
