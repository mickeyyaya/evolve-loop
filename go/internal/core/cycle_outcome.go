package core

import (
	"strings"
)

func preserveOnVerdict(finalVerdict string) bool {
	return finalVerdict == VerdictFAIL
}

func (o *Orchestrator) finalizeOutcome(lastPhaseVerdict, retroDecision string, shipped bool) string {
	if lastPhaseVerdict != VerdictSKIPPED {
		return lastPhaseVerdict
	}
	if shipped {
		return CycleOutcomeShippedViaBuild
	}
	if strings.Contains(retroDecision, "would-have-blocked") {
		return CycleOutcomeSkippedAuditAdvisory
	}
	return CycleOutcomeSkippedUnknown
}

func latchShippedState(cs *CycleState, phase Phase, verdict string) bool {
	if phase != PhaseShip || verdict != VerdictPASS {
		return false
	}
	cs.Shipped = true
	cs.ShipRecoveryCode = ""
	cs.ShipRecoveryConflicts = nil
	cs.AuditDeclineReason = ""
	return true
}
