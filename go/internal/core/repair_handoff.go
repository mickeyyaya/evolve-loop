package core

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/fixability"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeRepairHandoff signalcenter.Code = fixability.SignalCode
	RepairHandoffFile                   = "repair-handoff.json"

	repairHandoffReasonPrefix = auditDeclineReasonPrefix + "repair-handoff: "
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleRepair, CodeRepairHandoff, "the fixability check found a hand-off finding before a repair round (ADR-0158); fields.stage is shadow or enforce, fields.decision is none, would_stop (shadow) or stop (enforce: the cycle seals handed-off with no repair round), fields.reason is handoff_blocker on a stop or a would_stop, fields.blockers, fields.handoff and fields.lane list the finding ids, fields.console_items the console items the findings name and fields.salvage the worktree of the cycle")
}

func (o *Orchestrator) repairHandoff(cs CycleState, fb *phasecontract.FailureBlock, env retryEnvelope) (string, bool) {
	stage := o.workflowConfig.RepairFixability
	if stage == config.StageOff || !offersRepair(env) {
		return "", false
	}
	d := fixability.Classify(fixability.Input{
		Defects: fb.Defects, GateFailures: gateFailures(cs), OpenConsoleItems: o.openConsoleIDs(cs.WorkspacePath),
		Protected: o.protectedSurface, Salvage: cs.ActiveWorktree,
	})
	decision := fixability.DecisionNone
	switch {
	case d.Stop && stage == config.StageEnforce:
		decision = fixability.DecisionStop
	case d.Stop:
		decision = fixability.DecisionWould
	}
	if len(d.Handoff) > 0 {
		o.emitRepairHandoff(cs, stage, decision, d)
	}
	if decision != fixability.DecisionStop {
		return "", false
	}
	if err := atomicwrite.JSON(filepath.Join(cs.WorkspacePath, RepairHandoffFile), d); err != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN cycle %d: %s not written: %v\n", cs.CycleID, RepairHandoffFile, err)
	}
	return repairHandoffReasonPrefix + d.Reason + ": " + handoffSummary(d), true
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

func (o *Orchestrator) openConsoleIDs(workspace string) []string {
	inbox := filepath.Join(filepath.Dir(filepath.Dir(workspace)), "inbox")
	scan, err := inboxbatch.ScanDir(inbox)
	if err != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN fixability: the inbox %s is unreadable (%v); no console item can make a finding a hand-off finding\n", inbox, err)
		return nil
	}
	var ids []string
	for _, it := range fixability.OpenConsole(scan.Items, o.protectedSurface) {
		ids = append(ids, it.ID)
	}
	return ids
}

func handoffSummary(d fixability.Decision) string {
	parts := make([]string, 0, len(d.Blockers))
	for _, f := range d.Blockers {
		parts = append(parts, f.ID+" ("+f.Blocks+")")
	}
	return "blockers " + strings.Join(parts, ", ") + "; console items " + strings.Join(d.ConsoleItems, ", ") + "; salvage " + d.Salvage
}

func findingIDs(fs []fixability.Finding) string {
	ids := make([]string, 0, len(fs))
	for _, f := range fs {
		ids = append(ids, f.ID)
	}
	return strings.Join(ids, ",")
}

func (o *Orchestrator) emitRepairHandoff(cs CycleState, stage config.Stage, decision string, d fixability.Decision) {
	severity := signalcenter.SeverityWarn
	if !d.Stop {
		severity = signalcenter.SeverityInfo
	}
	o.signals.Emit(signalcenter.Event{
		Cycle: cs.CycleID, RunID: o.signalRunID(), Phase: string(PhaseAudit), Attempt: cs.AuditDispatches,
		Module: signalcenter.ModuleRepair, Origin: "Orchestrator.repairHandoff", Kind: signalcenter.KindPhaseOutcome,
		Severity: severity, Code: CodeRepairHandoff,
		Reason: "fixability (" + stage.String() + "): " + decision,
		Fields: map[string]string{
			"stage": stage.String(), fixability.FieldDecision: decision, fixability.FieldReason: d.Reason, fixability.FieldBlockers: findingIDs(d.Blockers),
			"handoff": findingIDs(d.Handoff), "lane": findingIDs(d.Lane), "console_items": strings.Join(d.ConsoleItems, ","), "salvage": d.Salvage,
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
