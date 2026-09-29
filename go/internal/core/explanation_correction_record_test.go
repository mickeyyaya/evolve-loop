package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func TestRecordExplanationCorrection_TheContinuationReadsTheDefects(t *testing.T) {
	ws := t.TempDir()

	if err := recordExplanationCorrection(ws, cycle1745Defects); err != nil {
		t.Fatalf("recordExplanationCorrection: %v", err)
	}

	var findings string
	stderr := captureStderr(t, func() {
		findings = readContinuationFindings(filepath.Join(ws, "audit-fail-reason.json"))
	})
	if strings.Contains(stderr, "unreadable") {
		t.Fatalf("the continuation still WARNs the findings artifact is unreadable:\n%s", stderr)
	}
	if !strings.HasPrefix(findings, "failed phase: audit\n") {
		t.Fatalf("findings must name the audit as the failed phase:\n%s", findings)
	}
	for _, defect := range cycle1745Defects {
		if !strings.Contains(findings, "- "+explanationNeedsCorrection+": "+defect) {
			t.Errorf("findings lack the classed defect %q:\n%s", defect, findings)
		}
	}
	phase, reasons := readAuditFailReason(ws)
	if phase != string(PhaseAudit) || len(reasons) != len(cycle1745Defects) {
		t.Fatalf("readAuditFailReason = (%q, %d reasons), want (audit, %d)", phase, len(reasons), len(cycle1745Defects))
	}
}

func TestRecordExplanationCorrection_TheNextAuditDispatchRetiresTheRecord(t *testing.T) {
	ws := t.TempDir()
	if err := recordExplanationCorrection(ws, cycle1745Defects); err != nil {
		t.Fatalf("recordExplanationCorrection: %v", err)
	}
	cs := CycleState{WorkspacePath: ws}

	resetFloorFailReason(&cs, PhaseAudit)

	if _, err := os.Stat(filepath.Join(ws, "audit-fail-reason.json")); !os.IsNotExist(err) {
		t.Fatalf("the re-audit must not inherit the corrected round's record: stat err = %v", err)
	}
}

func TestRecordExplanationRound_AnUnwritableWorkspaceWarnsNamingTheClass(t *testing.T) {
	cs := CycleState{CycleID: correctionCycle, WorkspacePath: filepath.Join(t.TempDir(), "absent")}

	stderr := captureStderr(t, func() {
		recordExplanationRound(cs, &phasecontract.FailureBlock{Defects: cycle1745Defects})
	})

	if !strings.Contains(stderr, "WARN cycle 1745 "+explanationNeedsCorrection) {
		t.Fatalf("a record the re-author will not see must be said on stderr, got:\n%s", stderr)
	}
}

func TestRecordExplanationCorrection_AnUnwritableWorkspaceIsAnError(t *testing.T) {
	ws := filepath.Join(t.TempDir(), "absent")

	if err := recordExplanationCorrection(ws, cycle1745Defects); err == nil {
		t.Fatal("a record the continuation cannot read must fail loudly, not silently")
	}
}
