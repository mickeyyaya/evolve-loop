package core

import (
	"strings"
)

func preserveOnVerdict(finalVerdict string) bool {
	return finalVerdict == VerdictFAIL
}

// finalizeOutcome translates a bare SKIPPED cycle verdict into a specific
// CycleOutcome label. PASS/FAIL/WARN pass through untouched.
//
// SHIPPED_VIA_BUILD requires THIS cycle's own ship latch (CycleState.Shipped,
// set by latchShippedState only when the ship phase PASSed and survived the
// deliverable review, on either dispatch root). It is never inferred from main
// HEAD movement: in fleet mode a sibling lane moves HEAD constantly, and an
// unrelated cycle with no ship of its own could otherwise be credited with a
// sibling's landing.
//
// See ADR-0100.
//
// A SKIPPED verdict without a ship keeps its no-work label so
// IsTriageNoWorkResult and the throughput recorder read the cycle truthfully.
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

// latchShippedState records the cycle's own ship PASS on the persisted cycle
// state and reports whether it did. Both dispatch roots call it right after the
// deliverable review approves a phase: the fresh loop (cyclerun_postreview.go)
// and the resume loop (resume_execution.go). The checkpoint is the latch's ONE
// home — the outcome label (finalizeOutcome) and the post-ship observer degrade
// (postShipObserverSkip) read it there — so a pause/resume after ship cannot
// lose the fact the way an in-memory field of one root would.
func latchShippedState(cs *CycleState, phase Phase, verdict string) bool {
	if phase != PhaseShip || verdict != VerdictPASS {
		return false
	}
	cs.Shipped = true
	cs.ShipRecoveryCode = "" // the recovery (if any) has landed; later re-entries are unrelated work
	cs.ShipRecoveryConflicts = nil
	cs.AuditDeclineReason = "" // the retro-routed retry (if any) shipped; the decline is spent
	return true
}
