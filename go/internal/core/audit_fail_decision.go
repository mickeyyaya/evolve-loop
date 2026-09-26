package core

import (
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// RetryAdjudicator proposes a disposition for an audit FAIL inside the legal envelope; a nil proposal yields the policy default.
type RetryAdjudicator interface {
	Adjudicate(cs CycleState, env retryEnvelope) *adjudication
}

func reentryPhase(a retryAction) (Phase, bool) {
	switch a {
	case retryActionRetryTDD:
		return PhaseTDD, true
	case retryActionRetryBuild:
		return PhaseBuild, true
	default:
		return PhaseRetro, false
	}
}

// decideAfterAuditFail consults deterministic evidence, then policy, then judgment among the actions those allowed.
// See ADR-0093.
func (o *Orchestrator) decideAfterAuditFail(cs CycleState) (Phase, string, *SystemFailureSignal) {
	d := buildFailureDossier(cs, VerdictFAIL, o.failurePolicy)
	_ = writeFailureDossier(cs.WorkspacePath, d) // per-cycle forensics; best-effort

	declared := ""
	if fb, ok := phasecontract.ReadFailureBlock(cs.WorkspacePath, string(PhaseAudit)); ok {
		declared = fb.Class
	}

	env := computeRetryEnvelope(retryEnvelopeInput{
		DeterministicFloorCandidate: d.FloorCandidate,
		DeclaredClass:               declared,
		Attempts:                    cs.AuditRepairAttempts,
		Policy:                      o.failurePolicy,
	})

	if env.Halt {
		// A halted cycle still gets its retro post-mortem; the signal is what stops the loop.
		return PhaseRetro, "audit-fail-floor: " + env.Reason, &SystemFailureSignal{
			Category: firstNonEmpty(d.FloorCandidate, declared),
			Level:    policy.LevelSystem,
			Evidence: env.Reason,
			Halt:     true,
		}
	}

	var proposal *adjudication
	if adjudicationNeeded(env) && o.retryAdjudicator != nil {
		proposal = o.retryAdjudicator.Adjudicate(cs, env)
	}
	action, clamped := clampAdjudication(env, proposal)

	suffix := ""
	switch {
	case clamped:
		suffix = " [adjudication clamped to the policy envelope]"
	case proposal != nil && proposal.Justification != "":
		// Surface the reasoning, not just the verdict word: the justification is the adjudicator's deliverable.
		suffix = " [adjudicated: " + proposal.Justification + "]"
	}

	if next, isRetry := reentryPhase(action); isRetry {
		o.emitAuditRepairDecision(cs, next, declared, env.Reason+suffix)
		return next, auditRepairReasonPrefix + string(action) + ": " + env.Reason + suffix, nil
	}
	o.emitAuditRepairDecision(cs, PhaseRetro, declared, env.Reason+suffix)
	return PhaseRetro, auditDeclineReasonPrefix + env.Reason + suffix, nil
}

// emitAuditRepairDecision emits one signal: WARN when the repair round is declined, INFO when granted.
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

// decisionOnlyEdge reports whether an edge exists only for decideAfterAuditFail to schedule. The routing
// advisor must never propose it: an advisor-routed re-entry would grant a retry the envelope refused, uncounted.
func decisionOnlyEdge(from, to Phase) bool {
	return from == PhaseAudit && (to == PhaseTDD || to == PhaseBuild)
}
