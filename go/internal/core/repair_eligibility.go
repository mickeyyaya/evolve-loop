package core

import "strings"

// legitRejection is the disposition vocabulary word meaning "the auditor was right
// and the defect is in the task's own work" — the only classification that can
// contradict an agent's own floor claim.
const legitRejection = "legit-rejection"

// auditRepairReasonPrefix tags the branch reason a repair grant emits. The
// emitter (decideAfterAuditFail) and the consumer (consumeAuditRepairGrant) key on
// this ONE constant: a literal on either side that drifts from the other would
// silently disable the bound, granting repairs that nothing counts.
const auditRepairReasonPrefix = "audit-repair: "

// auditDeclineReasonPrefix tags the branch reason decideAfterAuditFail emits
// when the envelope declines a direct repair and the cycle goes to retro.
const auditDeclineReasonPrefix = "audit-fail-decline: "

// CtxKeyAuditRepairFindings is the PhaseRequest.Context key carrying the audit's
// own fail-reason text into a repair re-dispatch. Exported and single-sourced
// because the SETTER lives in core and the READERS live in internal/phases/*:
// the sibling "continuation_findings" key is a bare literal repeated in two
// packages, which is a drift waiting to happen — this one cannot drift.
const CtxKeyAuditRepairFindings = "audit_repair_findings"

// CtxKeyShipErrorCode is the dispatch-context key carrying the ship error
// code a recovery is rebuilding from (retry_opts.go writes it; the debugger,
// the phaseio shadow and the standing-findings prompts read it).
const CtxKeyShipErrorCode = "ship_error_code"

// CtxKeyStandingAuditFindings carries the last audit's actionable findings
// into a tdd/build re-entry that follows a SHIP-error recovery: the audit
// passed (WARN) so no repair grant exists, yet the recovery rebuild is
// re-audited by the same rubric and every finding left unaddressed is named
// again as standing. A rejection grant outranks it.
const CtxKeyStandingAuditFindings = "standing_audit_findings"

// CtxKeyAuditDeclineReason carries the envelope's decline reason into a
// retro-routed re-entry so the prompt can name why no direct repair ran.
const CtxKeyAuditDeclineReason = "audit_decline_reason"

const CtxKeyExplanationReauthor = "explanation_reauthor_document"

// consumeAuditRepairGrant records decideAfterAuditFail's disposition on
// persisted cycle state — it is the ONE latch for both branches: a grant
// ("audit-repair: …") spends a retry attempt and marks the repair round
// active, so the audit's own findings are seeded into the re-dispatched
// tdd/build prompts; a decline ("audit-fail-decline: …") keeps the envelope's
// reason so a retro-routed re-entry can be told why no direct repair ran
// (retroRouted). Named for the grant it started as; both dispatch roots call
// it right after the decision.
func consumeAuditRepairGrant(cs *CycleState, reason string) {
	switch {
	case strings.HasPrefix(reason, auditRepairReasonPrefix):
		cs.AuditRepairAttempts++
		cs.AuditRepairActive = true
		cs.AuditDeclineReason = ""
	case strings.HasPrefix(reason, auditDeclineReasonPrefix):
		cs.AuditDeclineReason = strings.TrimPrefix(reason, auditDeclineReasonPrefix)
	}
}

// repairSeededPhase reports whether a repair brief can actually be acted on by
// this phase.
// Audit re-reads its own artifacts, so re-injecting its own rejection would be
// circular; retro already holds the full dossier.
func repairSeededPhase(p Phase) bool { return p == PhaseTDD || p == PhaseBuild }

// seedAuditRepairContext returns a COPY of ctx carrying the audit's own
// fail-reason text when a repair is in flight and the next phase can act on
// it — or, when no repair grant is active but a ship-error recovery is
// (CycleState.ShipRecoveryCode), the last audit's actionable findings as
// STANDING findings: the recovery rebuild is re-audited by the same rubric.
// All three routes derive from persisted cycle state, so the live dispatch
// loop and the crash-resume path cannot diverge. Absent/unreadable findings
// degrade to "no key", which the prompts treat as today's behaviour.
func seedAuditRepairContext(base map[string]string, next Phase, cs CycleState) map[string]string {
	if !repairSeededPhase(next) {
		return base
	}
	if cs.AuditRepairActive {
		return withContext(base, CtxKeyAuditRepairFindings, composeRepairBrief(cs))
	}
	if cs.ShipRecoveryCode == "" && !retroRouted(cs) {
		return base
	}
	out := withContext(base, CtxKeyStandingAuditFindings, auditorFindingsBrief(cs.WorkspacePath, cs.AuditDispatches))
	if cs.ShipRecoveryCode != "" && out[CtxKeyShipErrorCode] == "" { // a resumed dispatch has no snapshot of the code; the prompt names it from state
		out = withContext(out, CtxKeyShipErrorCode, cs.ShipRecoveryCode)
	}
	if cs.ShipRecoveryCode == "" {
		out = withContext(out, CtxKeyAuditDeclineReason, cs.AuditDeclineReason)
	}
	return out
}

// retroRouted reports whether the phase about to be dispatched re-enters the
// cycle straight after a retrospective that followed an audit-fail DECLINE —
// the envelope refused a direct repair (unrecognised class, budget,
// system-level class) and the retro's floor gates then adjudicated a retry.
// Both halves are required: a retro reached from a dispatch error or an
// exhausted correction ladder is not re-audited work owed the audit's
// findings (AuditDeclineReason is empty there), and a decline whose retro
// sealed the cycle never re-enters. Such a re-entry is re-audited by the same
// rubric, so it carries the standing findings exactly as a ship-error
// recovery does.
func retroRouted(cs CycleState) bool {
	n := len(cs.CompletedPhases)
	return cs.AuditDeclineReason != "" && n > 0 && cs.CompletedPhases[n-1] == string(PhaseRetro)
}

// StandingFindingsIntro is the ONE sentence the tdd and build prompts open the
// "Standing Audit Findings" section with: which route brought the cycle back
// to this phase after its audit had already spoken.
func StandingFindingsIntro(ctx map[string]string) string {
	if code := ctx[CtxKeyShipErrorCode]; code != "" {
		return "A ship-time error (" + code + ") sent this cycle back after its audit passed with findings."
	}
	return "A retrospective routed this cycle back after its audit FAILed and the retry envelope declined a direct repair (" + ctx[CtxKeyAuditDeclineReason] + ")."
}

// withContext returns base unchanged when value is empty, else a COPY of base
// carrying key=value (copy-on-write: the snapshot the dispatch loop keeps must
// never inherit a round's brief).
func withContext(base map[string]string, key, value string) map[string]string {
	if value == "" {
		return base
	}
	out := make(map[string]string, len(base)+1)
	for k, v := range base {
		out[k] = v
	}
	out[key] = value
	return out
}
