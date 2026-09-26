package core

import "slices"

// isAuthoritativePhase reports whether a completed phase's verdict is allowed
// to set the cycle's FinalVerdict: the resolved ship floor (tdd/build/audit
// or the configured override) plus ship, the phase that produces the shipped
// result.
func (o *Orchestrator) isAuthoritativePhase(phase Phase) bool {
	if phase == PhaseShip {
		return true
	}
	return slices.Contains(o.resolvedShipFloor(), string(phase))
}

// floorAlreadyCompleted reports whether any authoritative phase already
// appears in the completed-phase log. Callers pass cs.CompletedPhases, which
// already includes the phase currently being recorded — harmless, since a
// non-authoritative current phase never trips this.
func (o *Orchestrator) floorAlreadyCompleted(completed []string) bool {
	for _, p := range completed {
		if o.isAuthoritativePhase(Phase(p)) {
			return true
		}
	}
	return false
}

// recordFinalVerdict applies the floor-gated FinalVerdict update. An
// authoritative phase (or any phase before a floor verdict exists) sets
// FinalVerdict directly. A non-floor phase running after a floor verdict has
// been recorded must not clobber it: its non-PASS outcome is appended to
// result.VerdictsNotAdopted instead (a PASS needs no record). Pure apart from
// the append; safe to call from both dispatch loops.
func (o *Orchestrator) recordFinalVerdict(result *CycleResult, phase Phase, verdict string, floorAlreadyRecorded bool) {
	if o.isAuthoritativePhase(phase) || !floorAlreadyRecorded {
		result.FinalVerdict = verdict
		return
	}
	if verdict != VerdictPASS {
		result.VerdictsNotAdopted = append(result.VerdictsNotAdopted, VerdictNotAdopted{Phase: string(phase), Verdict: verdict})
	}
}

// nonFloorExhaustionDegrade decides whether a phase that exhausted its
// retries with a non-canonical verdict should degrade to SKIPPED+WARN instead
// of aborting the cycle. It degrades only a post-verdict non-floor phase —
// one running after the floor already passed. An authoritative phase, or any
// non-floor phase before the floor is established (scout/triage/intent,
// whose unparseable verdict must stay cycle-fatal), returns ok=false and
// keeps the abort.
func (o *Orchestrator) nonFloorExhaustionDegrade(phase Phase, workspace string, floorAlreadyRecorded bool) (PhaseResponse, bool) {
	if o.isAuthoritativePhase(phase) || !floorAlreadyRecorded {
		return PhaseResponse{}, false
	}
	return PhaseResponse{Phase: string(phase), Verdict: VerdictSKIPPED, ArtifactsDir: workspace}, true
}
