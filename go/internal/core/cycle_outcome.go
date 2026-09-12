package core

import (
	"strings"
)

func preserveOnVerdict(finalVerdict string) bool {
	return finalVerdict == VerdictFAIL
}

// isScoutEvalMaterialization reports whether a main-tree write is scout
// performing its documented eval-materialization contract. Scout writes the
// SELECTED slugs' evals to projectRoot/.evolve/evals/<slug>.md in the MAIN
// tree (internal/evalgate/materialization.go reads them there for Gate A), so
// that write is scout's JOB, not a deliverable escape. Without this carve-out
// a later cycle iterating the same coverage target re-materializes the same
// slug, MODIFYING the prior cycle's committed eval (soak-#6 cycle 318→319
// ledger-seal-io-coverage), and the tree-diff guard aborts the cycle. Scoped
// to scout + .evolve/evals/<slug>.md only: a code phase leaking an eval, or
// scout writing a non-.md file or any other deliverable (phases/, commit-
// prefix-scope.json) or a source file, all still fire the guard.
func isScoutEvalMaterialization(phase Phase, p string) bool {
	return phase == PhaseScout && strings.HasPrefix(p, ".evolve/evals/") && strings.HasSuffix(p, ".md")
}

// finalizeOutcome translates a bare SKIPPED cycle verdict into a specific
// CycleOutcome label. PASS/FAIL/WARN pass through untouched.
//
// SHIPPED_VIA_BUILD requires THIS cycle's own ship latch (CycleState.Shipped,
// set by latchShippedState only when the ship phase PASSed and survived the
// deliverable review, on either dispatch root). It is never
// inferred from main HEAD movement: in fleet mode a sibling lane moves HEAD
// constantly, and cycle 1630 — scout, triage, an honest empty commitment, no
// ship — was credited with a sibling's landing (ADR-0100, PR-3). A SKIPPED
// verdict without a ship keeps its no-work label so IsTriageNoWorkResult and
// the throughput recorder read the cycle truthfully.
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
	return true
}
