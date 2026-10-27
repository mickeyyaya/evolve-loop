package core

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeRepairHandoff   signalcenter.Code = cyclestate.RepairHandoffSignal
	RepairHandoffFile                     = "repair-handoff.json"
	RepairDecisionNone                    = cyclestate.RepairDecisionNone
	RepairDecisionWould                   = cyclestate.RepairDecisionWould
	RepairDecisionStop                    = cyclestate.RepairDecisionStop

	repairHandoffReasonPrefix = auditDeclineReasonPrefix + "repair-handoff: "
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleRepair, CodeRepairHandoff, "the fixability check found a hand-off finding before a repair round (ADR-0158); fields.stage is shadow or enforce, fields.decision is none, would_stop (shadow) or stop (enforce: the cycle seals handed-off with no repair round), fields.reason is handoff_blocker on a stop or a would_stop, fields.blockers, fields.handoff and fields.lane list the finding ids, fields.console_items the console items the findings name and fields.salvage the worktree of the cycle")
}

type RepairCheck struct {
	Defects      []string
	GateFailures []string
	InboxDir     string
	Salvage      string
}

type RepairFindings struct {
	Stop         bool     `json:"stop"`
	Reason       string   `json:"reason,omitempty"`
	Lane         []string `json:"lane,omitempty"`
	Handoff      []string `json:"handoff,omitempty"`
	Blockers     []string `json:"blockers,omitempty"`
	ConsoleItems []string `json:"console_items,omitempty"`
	Salvage      string   `json:"salvage,omitempty"`
}

func WithRepairFixability(check func(RepairCheck) RepairFindings) Option {
	return func(o *Orchestrator) { o.repairFixability = check }
}

func (o *Orchestrator) repairHandoff(cs CycleState, fb *phasecontract.FailureBlock, env retryEnvelope) (string, bool) {
	stage := o.workflowConfig.Repair.Fixability
	if o.repairFixability == nil || stage.Stage == policy.FixabilityOff || !offersRepair(env) {
		return "", false
	}
	d := o.repairFixability(RepairCheck{
		Defects: fb.Defects, GateFailures: gateFailures(cs),
		InboxDir: filepath.Join(filepath.Dir(filepath.Dir(cs.WorkspacePath)), "inbox"), Salvage: cs.ActiveWorktree,
	})
	decision := RepairDecisionNone
	switch {
	case d.Stop && stage.Enforced():
		decision = RepairDecisionStop
	case d.Stop:
		decision = RepairDecisionWould
	}
	if len(d.Handoff) > 0 {
		o.emitRepairHandoff(cs, decision, d)
	}
	if decision != RepairDecisionStop {
		return "", false
	}
	if err := atomicwrite.JSON(filepath.Join(cs.WorkspacePath, RepairHandoffFile), d); err != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN cycle %d: %s not written: %v\n", cs.CycleID, RepairHandoffFile, err)
	}
	return repairHandoffReasonPrefix + d.Reason + ": blockers " + strings.Join(d.Blockers, ", ") + "; console items " + strings.Join(d.ConsoleItems, ", ") + "; salvage " + d.Salvage, true
}

func offersRepair(env retryEnvelope) bool {
	for _, a := range env.Legal {
		if _, retry := reentryPhase(a); retry {
			return true
		}
	}
	return false
}

func gateFailures(cs CycleState) []string {
	out := append([]string(nil), cs.AuditFailReasons...)
	if _, _, msg, ok := readShipError(cs.WorkspacePath); ok {
		out = append(out, msg)
	}
	return out
}

func (o *Orchestrator) emitRepairHandoff(cs CycleState, decision string, d RepairFindings) {
	stage := o.workflowConfig.Repair.Fixability.Stage
	severity := signalcenter.SeverityWarn
	if !d.Stop {
		severity = signalcenter.SeverityInfo
	}
	o.signals.Emit(signalcenter.Event{
		Cycle: cs.CycleID, RunID: o.signalRunID(), Phase: string(PhaseAudit), Attempt: cs.AuditDispatches,
		Module: signalcenter.ModuleRepair, Origin: "Orchestrator.repairHandoff", Kind: signalcenter.KindPhaseOutcome,
		Severity: severity, Code: CodeRepairHandoff,
		Reason: "fixability (" + stage + "): " + decision,
		Fields: map[string]string{
			"stage": stage, cyclestate.RepairDecisionField: decision, cyclestate.RepairReasonField: d.Reason, cyclestate.RepairBlockersField: strings.Join(d.Blockers, ","),
			"handoff": strings.Join(d.Handoff, ","), "lane": strings.Join(d.Lane, ","), "console_items": strings.Join(d.ConsoleItems, ","), "salvage": d.Salvage,
		},
	})
}

func IsHandedOffResult(result CycleResult) bool {
	return result.FinalVerdict == VerdictFAIL && result.TerminationReason == CycleTerminationHandedOff
}

func (cr *cycleRun) sealHandedOff(result *CycleResult, timings []phaseTimingEntry, reason string) bool {
	if !strings.HasPrefix(reason, repairHandoffReasonPrefix) {
		return false
	}
	result.TerminationReason = CycleTerminationHandedOff
	item, ok := cr.consoleRouteItem(PhaseAudit)
	if ok && len(timings) > 0 {
		last := &timings[len(timings)-1]
		route := Diagnostic{Severity: cyclestate.SeverityError, Code: cyclestate.DiagCodeHandoffRoute, Subject: item, Message: "handed off: " + strings.TrimPrefix(reason, repairHandoffReasonPrefix)}
		last.Diagnostics = append([]Diagnostic{route}, last.Diagnostics...)
	}
	return true
}
