package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func auditFixRun(t *testing.T, stage string, fb *phasecontract.FailureBlock, opts ...Option) (*cycleRun, *[]signalcenter.Event) {
	t.Helper()
	root, ws := t.TempDir(), t.TempDir()
	report := "# Audit\n\n## Verdict\nFAIL\n\n" + phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL", fb) + "\n"
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	center, events := captureSignals()
	opts = append([]Option{WithWorkflowConfig(ladderWorkflow(stage)), WithSignalCenter(center)}, opts...)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), opts...)
	return &cycleRun{o: o, cycle: 2035, req: CycleRequest{ProjectRoot: root}, cs: CycleState{
		CycleID: 2035, WorkspacePath: ws, AuditRepairActive: true, AuditRepairAttempts: 1, AuditFailReasons: fb.Defects,
	}}, events
}

var prescribedSmallFix = &phasecontract.FailureBlock{
	Class:        "code-audit-fail",
	Defects:      []string{"H1: go/internal/core/retro_remediation_filter_test.go:29 inboxFiles counts every entry"},
	Prescription: []string{"Make inboxFiles in go/internal/core/retro_remediation_filter_test.go:29 count only *.json items"},
}

func TestAuditRepair_ShadowRecordsThatItWouldChooseAuditFixOncePerCycle(t *testing.T) {
	t.Parallel()
	cr, events := auditFixRun(t, policy.RetryLadderShadow, prescribedSmallFix)
	req := PhaseRequest{}
	cr.applyDispatchPolicy(PhaseBuild, &req)
	if req.Ladder.LadderTier() != "" || req.CorrectionDirective != "" {
		t.Fatalf("shadow changed the dispatch: %+v", req)
	}
	d := eventsWithCode(*events, CodeLadderAuditFix)
	if len(d) != 1 || len(eventsWithCode(*events, CodeLadderDecision)) != 1 {
		t.Fatalf("decisions = %+v", d)
	}
	f := d[0].Fields
	if f["audit_fix"] != auditFixWould || f["audit_fix_rule"] != retryladder.AuditFixRuleSmallPrescribed ||
		f["audit_fix_findings"] != "1" || f["audit_fix_prescriptions"] != "1" || f["audit_fix_files"] != "1" ||
		f["audit_fix_lines"] != "1" || f["audit_fix_code_lines"] != "0" || f["audit_fix_tokens"] != "12" || f["audit_fix_handler"] != "absent" || f["stage"] != policy.RetryLadderShadow {
		t.Fatalf("the decision lacks the audit_fix rule and its data: %+v", f)
	}
	if cr.cs.Ladder == nil || cr.cs.Ladder.AuditFix != auditFixWould {
		t.Fatalf("the cycle state does not record the audit_fix choice: %+v", cr.cs.Ladder)
	}

	cr.cs.AuditRepairAttempts = 2
	cr.applyDispatchPolicy(PhaseBuild, &PhaseRequest{})
	second := eventsWithCode(*events, CodeLadderAuditFix)
	if len(second) != 2 || second[1].Fields["audit_fix"] != auditFixDecline || second[1].Fields["audit_fix_rule"] != retryladder.AuditFixRuleUsed {
		t.Fatalf("a second repair round chose audit_fix again: %+v", second[len(second)-1].Fields)
	}
}

func TestAuditRepair_AnUnprescribedFindingKeepsTheRepairRung(t *testing.T) {
	t.Parallel()
	fb := &phasecontract.FailureBlock{Class: "code-audit-fail", Defects: []string{"H1: x", "M1: y"}, Prescription: []string{"fix x"}}
	cr, events := auditFixRun(t, policy.RetryLadderShadow, fb)
	cr.applyDispatchPolicy(PhaseTDD, &PhaseRequest{})
	f := eventsWithCode(*events, CodeLadderAuditFix)[0].Fields
	if f["audit_fix"] != auditFixDecline || f["audit_fix_rule"] != retryladder.AuditFixRuleUnprescribed {
		t.Fatalf("fields = %+v, want a declined audit_fix by the unprescribed rule", f)
	}
	if cr.cs.Ladder.AuditFix != "" {
		t.Fatalf("a declined audit_fix was marked used: %+v", cr.cs.Ladder)
	}
}

type stubRungHandler struct{}

func (stubRungHandler) Handle(context.Context, RungInput) (RungResult, error) {
	return RungResult{}, nil
}

func TestAuditRepair_EnforceNamesTheRegisteredRungHandler(t *testing.T) {
	t.Parallel()
	cr, events := auditFixRun(t, policy.RetryLadderEnforce, prescribedSmallFix, WithRungHandler(retryladder.RungAuditFix, stubRungHandler{}))
	cr.applyDispatchPolicy(PhaseBuild, &PhaseRequest{})
	f := eventsWithCode(*events, CodeLadderAuditFix)[0].Fields
	if f["audit_fix"] != auditFixChosen || f["audit_fix_handler"] != "registered" {
		t.Fatalf("fields = %+v, want audit_fix chosen with the registered handler", f)
	}
	if h, ok := cr.o.rungHandler(retryladder.RungAuditFix); !ok || h == nil {
		t.Fatalf("the audit_fix handler is not registered")
	}
	if res, err := (stubRungHandler{}).Handle(context.Background(), RungInput{Rung: retryladder.RungAuditFix}); err != nil || res.Handled {
		t.Fatalf("stub handler = %+v %v", res, err)
	}
}
