package core

import (
	"context"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	auditFixWould   = "would"
	auditFixChosen  = "chosen"
	auditFixDecline = "decline"
)

type RungInput struct {
	Rung      retryladder.Rung
	Cycle     int
	Workspace string
	Worktree  string
	Choice    retryladder.AuditFixChoice
	Failure   phasecontract.FailureBlock
}

type RungResult struct {
	Handled bool
	Reason  string
}

type RungHandler interface {
	Handle(ctx context.Context, in RungInput) (RungResult, error)
}

func WithRungHandler(rung retryladder.Rung, h RungHandler) Option {
	return func(o *Orchestrator) {
		if o.rungHandlers == nil {
			o.rungHandlers = map[retryladder.Rung]RungHandler{}
		}
		o.rungHandlers[rung] = h
	}
}

func (o *Orchestrator) rungHandler(rung retryladder.Rung) (RungHandler, bool) {
	h, ok := o.rungHandlers[rung]
	return h, ok
}

func (cr *cycleRun) recordAuditFix(phase Phase, fb *phasecontract.FailureBlock) string {
	cfg := cr.o.workflowConfig.RetryLadder
	estimate := retryladder.EstimateAuditFix(fb.Defects, fb.Prescription)
	choice := retryladder.ChooseAuditFix(retryladder.AuditFixInput{
		Findings: len(fb.Defects), Prescriptions: len(fb.Prescription), AllPrescribed: retryladder.AllPrescribed(fb.Defects, fb.Prescription),
		Estimate: estimate, Used: cr.cs.Ladder != nil && cr.cs.Ladder.AuditFix != "",
		Bounds: retryladder.AuditFixBounds{
			MaxFiles: cfg.AuditFix.MaxFiles, MaxLines: cfg.AuditFix.MaxLines, MaxCodeLines: cfg.AuditFix.MaxCodeLines,
			MaxTokens: cfg.AuditFix.MaxTokens, NoNewProductionLogic: cfg.AuditFix.NoNewProductionLogic,
		},
	})
	mark := auditFixDecline
	switch {
	case choice.Choose && cfg.Enforced():
		mark = auditFixChosen
	case choice.Choose:
		mark = auditFixWould
	}
	handler := "absent"
	if _, ok := cr.o.rungHandler(retryladder.RungAuditFix); ok {
		handler = "registered"
	}
	fields := map[string]string{
		"stage": cfg.Stage, "audit_fix": mark, "audit_fix_rule": choice.Rule, "audit_fix_handler": handler,
		"audit_fix_findings": strconv.Itoa(choice.Input.Findings), "audit_fix_prescriptions": strconv.Itoa(choice.Input.Prescriptions),
		"audit_fix_files": strconv.Itoa(estimate.Files), "audit_fix_lines": strconv.Itoa(estimate.Lines),
		"audit_fix_code_lines": strconv.Itoa(estimate.CodeLines), "audit_fix_tokens": strconv.Itoa(estimate.Tokens),
	}
	cr.o.signals.Emit(signalcenter.Event{
		Cycle: cr.cs.CycleID, RunID: cr.o.signalRunID(), Phase: string(phase), Module: signalcenter.ModuleLadder, Origin: originAuditRepair,
		Kind: signalcenter.KindLadderDecision, Severity: signalcenter.SeverityInfo, Code: CodeLadderAuditFix, Fields: fields,
		Reason: "retry ladder (" + cfg.Stage + ") audit_fix " + mark + " by rule " + choice.Rule,
	})
	if mark == auditFixDecline {
		return ""
	}
	return mark
}
