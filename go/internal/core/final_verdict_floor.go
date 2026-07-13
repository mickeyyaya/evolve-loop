package core

import "slices"

// final_verdict_floor.go — cycle-802 floor-gated FinalVerdict guard
// (retro-bridge-timeout-width10).
//
// Root cause: both the main dispatch loop (cyclerun_record.go) and the resume
// loop (resume.go) wrote `result.FinalVerdict = <phase verdict>` UNCONDITIONALLY
// at the end of every phase. So a POST-verdict, non-floor phase (retrospective,
// memo, the *-scans, router/advisor) failing under quota (exit=85) or artifact
// timeout (exit=81) clobbered an already-PASS audit verdict back to FAIL and
// zeroed the whole wave (waves 17-19 of the 2026-07-13 resume batch went 0/3).
//
// Fix: only an AUTHORITATIVE phase — a ship-floor phase (tdd/build/audit, or the
// user-configured Policy.FloorPhases override, resolved via resolvedShipFloor)
// or ship itself — may set FinalVerdict directly. Once one has, a non-floor
// phase's non-PASS outcome is preserved-around and recorded into
// CycleResult.SkippedPhases (surfaced in the dossier), never overwriting the
// floor verdict. Before any authoritative verdict exists (pre-audit phases,
// scout-only investigation cycles) legacy behavior is preserved: the phase's
// verdict stands, so no cycle loses its outcome.
//
// The floor-already-recorded predicate is DERIVED from cs.CompletedPhases (the
// persisted phase log) rather than a mutable flag, so the resume path — which
// re-enters mid-cycle after audit already completed in a prior session — gets
// the identical guard for free.

// isAuthoritativePhase reports whether a completed phase's verdict is allowed to
// set the cycle's FinalVerdict: the resolved ship floor (tdd/build/audit or the
// configured override) plus ship, the phase that produces the shipped result.
// The post-verdict non-floor phases (retrospective, memo, test-amplification,
// the secret-leak/flake-rerun/error-handling scans, router/advisor) are NOT
// authoritative and may not clobber a floor verdict.
func (o *Orchestrator) isAuthoritativePhase(phase Phase) bool {
	if phase == PhaseShip {
		return true
	}
	return slices.Contains(o.resolvedShipFloor(), string(phase))
}

// floorAlreadyCompleted reports whether any authoritative phase already appears
// in the completed-phase log. Callers pass cs.CompletedPhases (which, at both
// write sites, already includes the phase currently being recorded — harmless,
// since a non-authoritative current phase never trips this and an authoritative
// one is handled by isAuthoritativePhase directly).
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
// FinalVerdict directly. A non-floor phase running AFTER a floor verdict has
// been recorded must not clobber it: its non-PASS outcome is appended to
// result.SkippedPhases (a PASS needs no record — it never threatened the
// verdict). Pure apart from the append; safe to call from both dispatch loops.
func (o *Orchestrator) recordFinalVerdict(result *CycleResult, phase Phase, verdict string, floorAlreadyRecorded bool) {
	if o.isAuthoritativePhase(phase) || !floorAlreadyRecorded {
		result.FinalVerdict = verdict
		return
	}
	if verdict != VerdictPASS {
		result.SkippedPhases = append(result.SkippedPhases, SkippedPhase{Phase: string(phase), Reason: verdict})
	}
}

// nonFloorExhaustionDegrade decides whether a phase that exhausted its retries
// with a non-canonical verdict should degrade to SKIPPED+WARN instead of
// aborting the cycle (cycle-802 Task 3, subsuming advisory-phase-contract-
// degrade). It degrades ONLY a POST-verdict non-floor phase — a non-floor phase
// running after the floor already passed (retro/memo/scans/advisor). An
// authoritative phase, OR any non-floor phase BEFORE the floor is established
// (scout/triage/intent, whose unparseable verdict must stay cycle-fatal — you
// cannot proceed without a scout), returns ok=false and keeps the abort.
func (o *Orchestrator) nonFloorExhaustionDegrade(phase Phase, workspace string, floorAlreadyRecorded bool) (PhaseResponse, bool) {
	if o.isAuthoritativePhase(phase) || !floorAlreadyRecorded {
		return PhaseResponse{}, false
	}
	return PhaseResponse{Phase: string(phase), Verdict: VerdictSKIPPED, ArtifactsDir: workspace}, true
}
