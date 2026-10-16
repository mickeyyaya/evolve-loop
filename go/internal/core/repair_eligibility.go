package core

import (
	"os"
	"strings"
)

const legitRejection = "legit-rejection"

const auditRepairReasonPrefix = "audit-repair: "

const auditDeclineReasonPrefix = "audit-fail-decline: "

const CtxKeyAuditRepairFindings = "audit_repair_findings"

const CtxKeyShipErrorCode = "ship_error_code"

const CtxKeyStandingAuditFindings = "standing_audit_findings"

const CtxKeyAuditDeclineReason = "audit_decline_reason"

const CtxKeyExplanationReauthor = "explanation_reauthor_document"

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

func repairSeededPhase(p Phase) bool { return p == PhaseTDD || p == PhaseBuild }

func seedAuditRepairContext(base map[string]string, next Phase, cs CycleState) map[string]string {
	if !repairSeededPhase(next) {
		return base
	}
	if cs.AuditRepairActive {
		out := withContext(base, CtxKeyAuditRepairFindings, composeRepairBrief(cs, os.Stderr))
		return withContext(out, CtxKeyExplanationReauthor, explanationReauthorScope(next, cs))
	}
	if cs.ShipRecoveryCode == "" && !retroRouted(cs) {
		return base
	}
	out := withContext(base, CtxKeyStandingAuditFindings, auditorFindingsBrief(cs.WorkspacePath, cs.AuditDispatches, os.Stderr))
	if cs.ShipRecoveryCode != "" && out[CtxKeyShipErrorCode] == "" {
		out = withContext(out, CtxKeyShipErrorCode, cs.ShipRecoveryCode)
	}
	if cs.ShipRecoveryCode == "" {
		out = withContext(out, CtxKeyAuditDeclineReason, cs.AuditDeclineReason)
	}
	return out
}

func retroRouted(cs CycleState) bool {
	n := len(cs.CompletedPhases)
	return cs.AuditDeclineReason != "" && n > 0 && cs.CompletedPhases[n-1] == string(PhaseRetro)
}

func StandingFindingsIntro(ctx map[string]string) string {
	if code := ctx[CtxKeyShipErrorCode]; code != "" {
		return "A ship-time error (" + code + ") sent this cycle back after its audit passed with findings."
	}
	return "A retrospective routed this cycle back after its audit FAILed and the retry envelope declined a direct repair (" + ctx[CtxKeyAuditDeclineReason] + ")."
}

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
