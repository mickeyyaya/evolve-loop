package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestDecideAfterAuditFail_EmitsTheRepairDecision(t *testing.T) {
	auditReport := func(class string) string {
		return "# Audit\n\n## Verdict\nFAIL\n\n" + phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL",
			&phasecontract.FailureBlock{Class: class, Defects: []string{"EGPS: red_count=1 [x]"}}) + "\n"
	}
	run := func(t *testing.T, class string, attempts int) (Phase, []signalcenter.Event) {
		t.Helper()
		ws := t.TempDir()
		if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(auditReport(class)), 0o644); err != nil {
			t.Fatal(err)
		}
		c := signalcenter.New()
		var got []signalcenter.Event
		c.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
		o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(c))
		next, _, _ := o.decideAfterAuditFail(CycleState{CycleID: 1684, WorkspacePath: ws, AuditRepairAttempts: attempts})
		return next, got
	}

	next, got := run(t, "superseded-predicate-contradiction", 0)
	if next != PhaseRetro || len(got) != 1 || got[0].Code != CodeAuditRepairDeclined || got[0].Severity != signalcenter.SeverityWarn ||
		got[0].Kind != signalcenter.KindPhaseOutcome || got[0].Cycle != 1684 || got[0].Phase != "audit" {
		t.Fatalf("an unrecognised class declines → retro with ONE WARN naming the decision: next=%s events=%+v", next, got)
	}
	if got[0].Fields["reason"] == "" || got[0].Fields["next"] != "retro" {
		t.Errorf("the WARN carries the envelope's reason and the branch: %+v", got[0].Fields)
	}

	next, got = run(t, "code-audit-fail", 0)
	if next == PhaseRetro || len(got) != 1 || got[0].Code != CodeAuditRepairGranted || got[0].Severity != signalcenter.SeverityInfo {
		t.Fatalf("a task-level class within budget grants a repair round with ONE INFO: next=%s events=%+v", next, got)
	}
	if got[0].Fields["next"] != string(next) || got[0].Fields["attempt"] != "1" {
		t.Errorf("the INFO names the re-entry phase and the attempt about to be spent: %+v", got[0].Fields)
	}
}
