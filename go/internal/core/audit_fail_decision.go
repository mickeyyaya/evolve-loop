package core

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// audit_fail_decision.go — the disposition of an audit FAIL, decided AT THE AUDIT
// CHOKEPOINT.
//
// Before this, an audit FAIL always routed to retro, and retro carried three
// unrelated jobs: analysing the failure, classifying it, and GATING whether a
// retry could happen. That conflation is what made ADR-0092's repair reachable on
// only 3 of 16 failures — the retry depended on retro's prose, which is usually
// absent. Here the retry depends on the audit's OWN machine-readable class and the
// ADR-0072 policy table, both of which are always present.
//
// Retro is not removed; it is moved to where it belongs. It is now reached exactly
// when the disposition is DECLINE — the terminal learning step, once, off the retry
// path, so its 20-47 minutes are paid per CYCLE rather than per ATTEMPT.

// RetryAdjudicator proposes how to dispose of an audit FAIL, given the legal
// envelope. It is a Strategy: the caller receives an action whether or not one ran.
//
// It is deliberately allowed to return nil. Absence is the Null Object path and
// yields the policy default — no agent artifact is load-bearing for this decision.
type RetryAdjudicator interface {
	Adjudicate(cs CycleState, env retryEnvelope) *adjudication
}

// reentryPhase maps a retry action to the phase it re-enters. Kept beside the
// vocabulary it maps so a new action cannot be added without a home.
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

// decideAfterAuditFail returns the successor phase, an operator-readable reason,
// and a halting signal when the ADR-0072 floor binds.
//
// Order is the safety order: deterministic evidence first, policy second, judgment
// last, and judgment only among options the first two already allowed.
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
		// Retro still runs — a halted cycle deserves its post-mortem — but the
		// signal is what stops the loop.
		return PhaseRetro, "audit-fail-floor: " + env.Reason, &SystemFailureSignal{
			Category: firstNonEmpty(d.FloorCandidate, declared),
			Level:    policy.LevelSystem,
			Evidence: env.Reason,
			Halt:     true,
		}
	}

	// Judgment is consulted only where more than one action is legal.
	var proposal *adjudication
	if adjudicationNeeded(env) && o.retryAdjudicator != nil {
		proposal = o.retryAdjudicator.Adjudicate(cs, env)
	}
	action, clamped := clampAdjudication(env, proposal)

	// One suffix, computed once: a clamp and an adjudication are mutually
	// exclusive outcomes of the same proposal, and writing the pair out twice
	// (once per arm) is how the two arms drift.
	suffix := ""
	switch {
	case clamped:
		suffix = " [adjudication clamped to the policy envelope]"
	case proposal != nil && proposal.Justification != "":
		// Surface the REASONING, not just the verdict word. clampAdjudication
		// rejects an unjustified proposal precisely because the justification is
		// this phase's deliverable; requiring it and then discarding it is the
		// defect shape ADR-0092's Incoherent flag had.
		suffix = " [adjudicated: " + proposal.Justification + "]"
	}

	if next, isRetry := reentryPhase(action); isRetry {
		return next, auditRepairReasonPrefix + string(action) + ": " + env.Reason + suffix, nil
	}
	return PhaseRetro, "audit-fail-decline: " + env.Reason + suffix, nil
}

// firstNonEmpty returns the first non-empty string, or "" if all are empty.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// decisionOnlyEdge reports whether an edge exists SOLELY for a deterministic
// decision to schedule, and must therefore never be proposable by the routing
// advisor.
//
// audit→tdd and audit→build are the audit-FAIL re-entry edges. They must be in
// the legality graph so decideAfterAuditFail can schedule them (and so the graph
// stays single-sourced with phase-registry.json and the trust anchor) — but the
// advisor validates its own proposals through that same graph. Left proposable,
// it could grant a retry the envelope refused, on a path that never spends the
// budget: consumeAuditRepairGrant keys on the audit-repair reason prefix, which
// only decideAfterAuditFail emits, so an advisor-routed re-entry would be
// UNCOUNTED and bounded only by defaultMaxPhaseIterations. It could equally route
// backwards after a halt, or override audit→ship on a PASSING audit.
//
// The adjudicator cannot widen the envelope; without this, the router could widen
// it around the adjudicator. Same authority question, different agent.
func decisionOnlyEdge(from, to Phase) bool {
	return from == PhaseAudit && (to == PhaseTDD || to == PhaseBuild)
}
