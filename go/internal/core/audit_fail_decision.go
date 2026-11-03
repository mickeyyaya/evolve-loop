package core

import (
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type RetryAdjudicator interface {
	Adjudicate(cs CycleState, env retryEnvelope) *adjudication
}

func reentryPhase(a retryAction) (Phase, bool) {
	switch a {
	case retryActionRetryTDD:
		return PhaseTDD, true
	case retryActionRetryBuild, retryActionReauthorExplanation:
		return PhaseBuild, true
	default:
		return PhaseRetro, false
	}
}

func (o *Orchestrator) decideAfterAuditFail(cs CycleState) (Phase, string, *SystemFailureSignal) {
	d := o.failureDossier(cs, VerdictFAIL)
	o.writeDossier(cs, d)

	env, fb := o.auditFailEnvelope(cs, d.FloorCandidate)
	if d.BuildDeclared != nil && !env.Halt {
		env = declineOnly("the build declared system-level class " + d.BuildDeclared.Class +
			" after the gate remedy round, and the audited tree did not change; no rebuild repairs it, so the item is deferred to a pipeline repair")
	}
	declared := fb.Class

	if env.Halt {
		return PhaseRetro, "audit-fail-floor: " + env.Reason, &SystemFailureSignal{
			Category: firstNonEmpty(d.FloorCandidate, declared),
			Level:    policy.LevelSystem,
			Evidence: env.Reason,
			Halt:     true,
		}
	}

	if reason, stop := o.repairHandoff(cs, fb, env); stop {
		o.emitAuditRepairDecision(cs, PhaseRetro, declared, strings.TrimPrefix(reason, auditDeclineReasonPrefix))
		return PhaseRetro, reason, nil
	}

	var proposal *adjudication
	if adjudicationNeeded(env) && o.retryAdjudicator != nil {
		proposal = o.retryAdjudicator.Adjudicate(cs, env)
	}
	action, clamped := clampAdjudication(env, proposal)

	suffix := adjudicationSuffix(clamped, proposal)

	stopReason := o.consultAuditRepairConvergence(cs)
	next, isRetry := reentryPhase(action)
	if isRetry && stopReason == "" {
		o.emitAuditRepairDecision(cs, next, declared, env.Reason+suffix)
		return next, auditRepairReasonPrefix + string(action) + ": " + env.Reason + suffix, nil
	}
	reason := env.Reason + suffix
	if isRetry {
		reason = stopReason + ": " + reason
	}
	o.emitAuditRepairDecision(cs, PhaseRetro, declared, reason)
	return PhaseRetro, auditDeclineReasonPrefix + reason, nil
}

func adjudicationSuffix(clamped bool, proposal *adjudication) string {
	switch {
	case clamped:
		return " [adjudication clamped to the policy envelope]"
	case proposal != nil && proposal.Justification != "":
		return " [adjudicated: " + proposal.Justification + "]"
	}
	return ""
}

func (o *Orchestrator) auditFailEnvelope(cs CycleState, floorCandidate string) (retryEnvelope, *phasecontract.FailureBlock) {
	fb, declared := phasecontract.ReadFailureBlock(cs.WorkspacePath, string(PhaseAudit))
	gateDerived := false
	if !declared {
		fb, gateDerived = gateFailureBlock(cs.AuditFailReasons)
	}
	if fb == nil {
		fb = &phasecontract.FailureBlock{}
	}
	env := computeRetryEnvelope(retryEnvelopeInput{
		DeterministicFloorCandidate: floorCandidate,
		DeclaredClass:               fb.Class,
		Attempts:                    cs.AuditRepairAttempts,
		Policy:                      o.failurePolicy,
	})
	if gateDerived {
		env = gateRemediationEnvelope(env)
	}
	if _, explanationOnly := explanationCorrectionDocument(cs, fb); explanationOnly {
		env = explanationCorrectionEnvelope(env)
	}
	return env, fb
}

func (o *Orchestrator) emitAuditRepairDecision(cs CycleState, next Phase, declared, reason string) {
	e := signalcenter.Event{
		Cycle: cs.CycleID, RunID: o.signalRunID(), Phase: string(PhaseAudit), Attempt: cs.AuditDispatches,
		Module: signalcenter.ModuleOrchestrator, Origin: "Orchestrator.decideAfterAuditFail",
		Kind:   signalcenter.KindPhaseOutcome,
		Fields: map[string]string{"next": string(next), "reason": reason, "declared_class": declared},
	}
	if next == PhaseRetro {
		e.Severity, e.Code = signalcenter.SeverityWarn, CodeAuditRepairDeclined
		e.Reason = "audit FAIL earned no repair round: " + reason
	} else {
		e.Severity, e.Code = signalcenter.SeverityInfo, CodeAuditRepairGranted
		e.Fields["attempt"] = strconv.Itoa(cs.AuditRepairAttempts + 1)
		e.Reason = "audit FAIL → repair round via " + string(next) + " (attempt " + e.Fields["attempt"] + "): " + reason
	}
	o.signals.Emit(e)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func decisionOnlyEdge(from, to Phase) bool {
	return from == PhaseAudit && (to == PhaseTDD || to == PhaseBuild || to == PhaseAudit)
}
