package core

import (
	"path/filepath"
	"strings"
)

// repair_eligibility.go — the in-cycle repair BOOKKEEPING: the durable attempt
// counter, the grant primitive, and the findings-injection seam.
//
// Its former eligibility RULE is gone (ADR-0093). Retries are now decided at the
// audit chokepoint from the audit's own declared failure class and the ADR-0072
// policy table — see audit_fail_decision.go and retry_envelope.go. One retry
// authority, not two: the deleted rule was a second mechanism built beside a
// declarative policy that had always declared the same cap and that nothing read.
//
// What survives here is the machinery a retry still needs wherever it is decided:
// a bound that outlives a crash, one primitive that spends it, and the seam that
// hands the rebuilding agent the audit's own findings.
//
// The legitRejection vocabulary word below is retained for the CORROBORATION half
// of ADR-0092, which is still live in applyFailureDecisionFloor: an agent-authored
// floor claim contradicted by both the deterministic evidence and the agent's own
// disposition does not halt. That narrowing is unchanged; only its retry-granting
// half was removed.

// legitRejection is the disposition vocabulary word meaning "the auditor was right
// and the defect is in the task's own work" — the only classification that can
// contradict an agent's own floor claim.
const legitRejection = "legit-rejection"

// auditRepairReasonPrefix tags the branch reason a repair grant emits. The
// emitter (decideAfterAuditFail) and the consumer (consumeAuditRepairGrant) key on
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
		cs.AuditRepairActive = true
	}
}

// repairSeededPhase reports whether a repair brief can actually be acted on by
// this phase.
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
func seedAuditRepairContext(base map[string]string, next Phase, cs CycleState) map[string]string {
	if !cs.AuditRepairActive || !repairSeededPhase(next) {
		return base
	}
	findings := readContinuationFindings(filepath.Join(cs.WorkspacePath, "audit-fail-reason.json"))
	if findings == "" {
		return base
	}
	out := make(map[string]string, len(base)+1)
	for k, v := range base {
		out[k] = v
	}
	out[CtxKeyAuditRepairFindings] = findings
	return out
}
