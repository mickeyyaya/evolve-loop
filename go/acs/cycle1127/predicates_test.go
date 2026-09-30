//go:build acs

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

func TestC1127_004_ConflictRecordReachesDossierPlumbing(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestVerdictConflict_ErrorDiagnosticReachesAuditFailReasons|"+
			"TestVerdictConflict_WarningSeverityWouldBeDropped")
	if !ok {
		t.Errorf("RED: the verdict-conflict record does not flow through the existing "+
			"cyclestate.ErrorMessages → AuditFailReasons → dossier chain — an unread record is "+
			"an unfixed defect:\n%s", tail(out, 40))
	}
}
