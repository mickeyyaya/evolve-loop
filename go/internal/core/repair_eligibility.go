package core

import (
	"path/filepath"
	"strconv"
	"strings"
)

// repair_eligibility.go — the audit-repair corroboration rule.
//
// An audit FAIL is terminal today: statemachine.go gives PhaseAudit exactly two
// successors (Ship on PASS/WARN, Retro on FAIL), so a rejected cycle pays a full
// teardown — worktree preservation, a 20-47 minute retrospective, continuation
// minting, and a fresh cycle's scout+triage+localization — to fix what a rebuild
// could often have addressed in place. This rule is the single decision that
// makes a bounded in-cycle repair possible: is THIS failure repairable?
//
// It is a pure function on purpose. Every I/O concern (reading the dossier,
// resolving which categories are policy floors) is inverted into the input, so
// the rule is exhaustively table-testable and has exactly one place to change.
//
// WHY IT IS SHAPED THIS WAY. Wave-3 cycles 1572/1573/1574 each halted under
// applyFailureDecisionFloor gate 2 — the agent-authored failure-decision.json
// category "infra-systemic". But all three failure-dossier.json record
// floor_candidate:"" (the DETERMINISTIC gate never fired), while the SAME retro
// agent's disposition.json recorded legitimacy:"legit-rejection". One agent
// contradicted itself inside one cycle and the prose half won, halting three
// cycles that two deterministic signals called task-level. That is the
// proxy-as-verdict class of docs/incidents/2026-08-12-proxy-as-verdict-findings.md.
//
// What this rule does NOT do: weaken ADR-0072. A deterministic floor candidate
// is absolute here, exactly as it is in gate 1. The only authority narrowed is
// prose contradicted by the deterministic evidence AND by the agent's own
// disposition.
type repairEligibilityInput struct {
	// DeterministicFloorCandidate is failureDossier.FloorCandidate — the
	// machine-computed floor category. Non-empty ⇒ no repair, unconditionally.
	DeterministicFloorCandidate string
	// AgentClaimedFloor reports whether failure-decision.json's category is a
	// policy floor. Resolved by the caller (policy is not this rule's concern);
	// used only to record an incoherence, never to grant repair.
	AgentClaimedFloor bool
	// Legitimacy is disposition.json's legitimacy field, validated against the
	// single-sourced validLegitimacy vocabulary in disposition_gate.go.
	Legitimacy string
	// Attempts is how many repairs this cycle has already dispatched.
	Attempts int
	// MaxAttempts is the configured cap (workflow.max_audit_repair_attempts).
	// Zero disables repair — the off switch is configuration, not a flag.
	MaxAttempts int
}

// repairEligibility is the decision. Reason is always populated: a decision an
// operator cannot read is a decision they cannot audit.
type repairEligibility struct {
	Eligible bool
	// Incoherent records that the agent claimed a floor category while the
	// deterministic gate was silent AND its own disposition said task-level.
	// It is a forensics signal about classifier disagreement, deliberately
	// distinct from Eligible — see the dedicated test.
	Incoherent bool
	Reason     string
}

// legitRejection is the one disposition vocabulary word that describes "the
// auditor was right and the defect is in the task's own work" — the only
// classification a rebuild can actually address. Declared here rather than
// inlined so the link to validLegitimacy is explicit at the point of use.
const legitRejection = "legit-rejection"

// repairRecoveryKey is the phase-catalog Recovery.Targets key for the repair
// branch. Routing through recoveryTarget keeps the destination CONFIG-selected
// (spec.Recovery.Targets[key]) with PhaseTDD only as the literal fallback, and
// leaves CanTransition as the legality constraint — the same contract every
// other control-phase recovery already uses. No new mechanism, and no new edge
// in the transition table: Retro→TDD is already legal.
const repairRecoveryKey = "REPAIR_RETRY"

// auditRepairReasonPrefix tags the branch reason a repair grant emits. The
// emitter (decideAfterRetro) and the consumer (consumeAuditRepairGrant) key on
// this ONE constant: a literal on either side that drifts from the other would
// silently disable the bound, granting repairs that nothing counts.
const auditRepairReasonPrefix = "audit-repair: "

// CtxKeyAuditRepairFindings is the PhaseRequest.Context key carrying the audit's
// own fail-reason text into a repair re-dispatch. Exported and single-sourced
// because the SETTER lives in core and the READERS live in internal/phases/*:
// the sibling "continuation_findings" key is a bare literal repeated in two
// packages, which is a drift waiting to happen — this one cannot drift.
const CtxKeyAuditRepairFindings = "audit_repair_findings"

// consumeAuditRepairGrant spends one repair attempt when the retro branch
// granted one. It mirrors consumeBookkeepingRegradeGrant exactly — the ONE
// primitive every branch surface calls (the live loop in cyclerun_record and
// the crash-resume path in resume.go), so the bound cannot drift out of one of
// them. A resume that skipped this would hand a resumed cycle a fresh,
// unbounded repair budget.
func consumeAuditRepairGrant(cs *CycleState, reason string) {
	if strings.HasPrefix(reason, auditRepairReasonPrefix) {
		cs.AuditRepairAttempts++
	}
}

// decideRepairEligibility applies the corroboration rule. Conservative by
// construction: absence of evidence never grants repair.
func decideRepairEligibility(in repairEligibilityInput) repairEligibility {
	// Gate 1 parity. A deterministic floor candidate ends the conversation —
	// and it ends it BEFORE the incoherence check, because an agent that agrees
	// with the deterministic evidence is not contradicting anything.
	if in.DeterministicFloorCandidate != "" {
		return repairEligibility{
			Reason: "deterministic floor candidate " + in.DeterministicFloorCandidate + "; repair cannot outrank ADR-0072 gate 1",
		}
	}

	// The agent claimed a floor while the deterministic gate stayed silent.
	// Whether that is a contradiction depends on the agent's OWN disposition,
	// checked below; record the claim now so both exits can report it.
	contradicted := in.AgentClaimedFloor && in.Legitimacy == legitRejection

	if !validLegitimacy[in.Legitimacy] {
		reason := "disposition legitimacy absent or out-of-vocabulary"
		if in.Legitimacy != "" {
			reason = "disposition legitimacy " + in.Legitimacy + " is out-of-vocabulary"
		}
		return repairEligibility{Reason: reason + "; absence of evidence does not grant repair"}
	}
	if in.Legitimacy != legitRejection {
		return repairEligibility{
			Reason: "disposition legitimacy " + in.Legitimacy + " is not a task-level rejection; a rebuild cannot address it",
		}
	}
	if in.Attempts >= in.MaxAttempts {
		return repairEligibility{
			Incoherent: contradicted,
			Reason:     "repair attempts exhausted (" + strconv.Itoa(in.Attempts) + "/" + strconv.Itoa(in.MaxAttempts) + ")",
		}
	}

	reason := "task-level rejection, attempt " + strconv.Itoa(in.Attempts+1) + "/" + strconv.Itoa(in.MaxAttempts)
	if contradicted {
		reason += "; agent claimed a floor category contradicted by an empty deterministic candidate and its own legit-rejection disposition"
	}
	return repairEligibility{Eligible: true, Incoherent: contradicted, Reason: reason}
}

// repairSeededPhases are the phases a repair brief can actually be acted on.
// Audit re-reads its own artifacts, so re-injecting its own rejection would be
// circular; retro already holds the full dossier.
func repairSeededPhase(p Phase) bool { return p == PhaseTDD || p == PhaseBuild }

// seedAuditRepairContext returns a COPY of ctx carrying the audit's own
// fail-reason text when a repair is in flight and the next phase can act on it.
//
// It derives from PERSISTED cycle state (AuditRepairAttempts) rather than being
// pushed at grant time, so the live dispatch loop and the crash-resume path
// cannot diverge: one rule, reading a field that survives both. Copying rather
// than mutating matters — the dispatch loop reuses one ctxSnap map across every
// iteration of the cycle, so an in-place write would leak a stale repair brief
// into phases that never asked for it.
//
// Absent/unreadable findings degrade to "no key", which the prompts treat as
// today's behaviour; readContinuationFindings already warns loudly on that path
// so the operator can tell "none existed" from "we looked in the wrong place".
func seedAuditRepairContext(ctx map[string]string, next Phase, cs CycleState) map[string]string {
	if cs.AuditRepairAttempts <= 0 || !repairSeededPhase(next) {
		return ctx
	}
	findings := readContinuationFindings(filepath.Join(cs.WorkspacePath, "audit-fail-reason.json"))
	if findings == "" {
		return ctx
	}
	out := make(map[string]string, len(ctx)+1)
	for k, v := range ctx {
		out[k] = v
	}
	out[CtxKeyAuditRepairFindings] = findings
	return out
}
