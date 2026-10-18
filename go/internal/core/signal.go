package core

import (
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodePhaseVerdictFail    signalcenter.Code = "ORCHESTRATOR_PHASE_VERDICT_FAIL"
	CodePhaseVerdictWarn    signalcenter.Code = "ORCHESTRATOR_PHASE_VERDICT_WARN"
	CodePhaseAborted        signalcenter.Code = "ORCHESTRATOR_PHASE_ABORTED"
	CodeAuditRepairDeclined signalcenter.Code = "ORCHESTRATOR_AUDIT_REPAIR_DECLINED"
	CodeAuditRepairGranted  signalcenter.Code = "ORCHESTRATOR_AUDIT_REPAIR_GRANTED"

	CodeExplanationReauthorRouted signalcenter.Code = "ORCHESTRATOR_EXPLANATION_REAUTHOR_ROUTED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodePhaseVerdictFail, "a phase recorded verdict FAIL; the reason is the phase's own error-severity diagnostics")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodePhaseVerdictWarn, "a phase recorded verdict WARN; the reason carries its error-severity diagnostics, if any")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeAuditRepairDeclined, "an audit FAIL earned no repair round and the cycle goes to retro; fields.reason is the retry envelope's verdict (unrecognised class, budget spent, system-level or non-retry class, no class declared), fields.declared_class the audit's class: its report's own, else the one its deterministic gates' diagnoses derive")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeAuditRepairGranted, "an audit FAIL earned a repair round; fields.next is the re-entry phase (tdd | build), fields.attempt the repair attempt about to be spent, fields.reason the envelope's basis")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeExplanationReauthorRouted, "a post-Build source writer left the Build explanation's content stale (a cited path no longer in the diff, Changed Areas drift) and the cycle routed back to Build for the owner to re-author it instead of aborting — fields.next is the re-entry phase; fields.charged is false when every legal successor of the writer already leads to Build (tdd), so the round costs nothing, and true when it spends one code-audit-fail attempt, with fields.attempt the attempt spent and fields.envelope the budget's reason; the reason is the validator's verdict")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodePhaseAborted, "the cycle aborted after this phase's outcome (review reject, guard, persistence); the abort reason is in fields.abort_reason")
}

func WithSignalCenter(c *signalcenter.Center) Option {
	return func(o *Orchestrator) {
		if c == nil {
			return
		}
		o.signals = c
		o.signalSummary = signalcenter.NewSummary()
		c.Subscribe(o.observeSignal)
	}
}

func (o *Orchestrator) SignalCenterWired() bool { return o.signals != nil }

func (o *Orchestrator) SignalSummary() signalcenter.Summary {
	o.signalMu.Lock()
	defer o.signalMu.Unlock()
	if o.signalSummary == nil {
		return signalcenter.NewSummary().Snapshot()
	}
	return o.signalSummary.Snapshot()
}

func (o *Orchestrator) observeSignal(e signalcenter.Event) {
	o.signalMu.Lock()
	defer o.signalMu.Unlock()
	o.signalSummary.Observe(e)
}

func (o *Orchestrator) emitPhaseDispatched(cycle int, runID string, phase Phase) {
	o.signals.Emit(signalcenter.Event{
		Cycle: cycle, RunID: runID, Phase: string(phase),
		Module: signalcenter.ModuleOrchestrator, Origin: "Orchestrator.emitPhaseDispatched",
		Kind: signalcenter.KindPhaseDispatched, Severity: signalcenter.SeverityInfo,
		Reason: string(phase) + " dispatched",
	})
}

func (o *Orchestrator) emitPhaseOutcome(cycle int, out recovery.PhaseOutcome) {
	e := signalcenter.Event{
		Cycle: cycle, RunID: o.signalRunID(), Phase: out.Phase, Attempt: out.AttemptCount,
		Module: signalcenter.ModuleOrchestrator, Origin: "Orchestrator.recordPhaseOutcome",
		Kind: signalcenter.KindPhaseOutcome, Severity: signalcenter.SeverityInfo,
		Reason: out.Phase + " " + verdictReason(out.Verdict, out.Diagnostics),
		Fields: map[string]string{
			"verdict": out.Verdict, "archetype": out.Archetype, "duration_ms": strconv.FormatInt(out.DurationMS, 10),
		},
	}
	if codes := cyclestate.ErrorCodes(out.Diagnostics); len(codes) > 0 {
		e.Fields["diagnostic_codes"] = strings.Join(codes, ",")
	}
	switch {
	case out.AbortReason != "":
		e.Kind, e.Severity, e.Code = signalcenter.KindPhaseAborted, signalcenter.SeverityWarn, CodePhaseAborted
		e.Reason = out.Phase + " aborted after verdict=" + out.Verdict + ": " + out.AbortReason
		e.Fields["abort_reason"] = out.AbortReason
	case out.Verdict == VerdictFAIL:
		e.Severity, e.Code = signalcenter.SeverityWarn, CodePhaseVerdictFail
	case out.Verdict == VerdictWARN:
		e.Severity, e.Code = signalcenter.SeverityWarn, CodePhaseVerdictWarn
	}
	o.signals.Emit(e)
}

const CodeGateCorrection signalcenter.Code = "ORCHESTRATOR_GATE_CORRECTION"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeGateCorrection, "the correction ladder ran a rung after a gate rejection — fields name the correction ordinal, the budget (max), the rung, the CLI re-dispatched on and whether that CLI was escalated; the reason is the rejection being corrected")
}

type gateCorrection struct {
	origin       string
	cycle        int
	phase        Phase
	correction   int
	max          int
	rung         string
	cli          string
	escalated    bool
	salvageRetry bool
	reason       string
}

func (o *Orchestrator) emitGateCorrection(gc gateCorrection) {
	o.signals.Emit(signalcenter.Event{
		Cycle: gc.cycle, RunID: o.signalRunID(), Phase: string(gc.phase),
		Module: signalcenter.ModuleOrchestrator, Origin: gc.origin, Kind: signalcenter.KindGateCorrected,
		Severity: signalcenter.SeverityInfo, Code: CodeGateCorrection, Reason: gc.reason,
		Fields: map[string]string{
			"correction": strconv.Itoa(gc.correction), "max": strconv.Itoa(gc.max), "rung": gc.rung, "cli": gc.cli,
			"escalated": strconv.FormatBool(gc.escalated), "salvage_retry": strconv.FormatBool(gc.salvageRetry),
		},
	})
}
