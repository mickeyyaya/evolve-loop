package core

import (
	"fmt"
	"os"
)

// completeCycle is the terminal lifecycle for both entrypoints. Resume must
// reconcile the verdict and preserve its learning before claiming completion.
func (cr *cycleRun) completeCycle() error {
	cr.recordPlannedNoWorkOutcome()
	// Post-loop finalization (verdict reclassification, silent-no-ship warn,
	// throughput, worktree-preserve decision, state persist) → finalizeCycle.
	// preserveWorktree is threaded back so the exit defer (registered above)
	// observes it; cycleCompletedNormally is set only on a clean persist.
	preserve, ferr := cr.o.finalizeCycle(cr.ctx, cr.cs, cr.cycle, cr.preCycleHEAD, cr.req.ProjectRoot, &cr.result, &cr.state, cr.phaseTimings)
	if preserve {
		cr.preserveWorktree = true
	}
	if ferr != nil {
		return ferr
	}
	cr.cycleCompletedNormally = true
	// ADR-0101 S2a: the cycle's event stream ends here on both roots —
	// system.failure (if the floors attached one) then cycle.sealed; the
	// abnormal path seals from abnormalEpilogue.
	cr.emitCycleClose(cr.result, "cycleRun.completeCycle")
	// ADR-0055: emit this completed cycle's closeout dossier to
	// <ProjectRoot>/knowledge-base/cycles/cycle-N.json. Best-effort — the cycle
	// has already finalized, so a closeout-artifact write error must not fail it
	// (presence is enforced separately by `evolve dossier verify` against the
	// policy floor). Goal text comes from Context["goal"]; falls back to the goal
	// hash so the dossier's required Goal is never blank.
	dossierGoal := cr.req.Context["goal"]
	if dossierGoal == "" {
		dossierGoal = cr.req.GoalHash
	}
	if derr := writeCycleDossier(cr.o.gitMutationLock, cycleDossierParams{
		ProjectRoot:        cr.req.ProjectRoot,
		WorkspacePath:      cr.cs.WorkspacePath,
		Cycle:              cr.cycle,
		Goal:               dossierGoal,
		RunID:              cr.cs.RunID,
		Outcome:            cr.result.FinalVerdict,
		SystemFailure:      cr.result.SystemFailure,
		SkippedPhases:      cr.result.SkippedPhases,
		VerdictsNotAdopted: cr.result.VerdictsNotAdopted,
		SpineFailOpens:     cr.result.SpineFailOpens,
		PhaseTimings:       cr.flushPhaseTimings(),
	}); derr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d: closeout dossier not written (non-fatal): %v\n", cr.cycle, derr)
	}
	return nil
}

// recordPlannedNoWorkOutcome applies an already-authorized host disposition.
// The terminal selector owns authorization; closeout only validates that no
// later phase or earlier implementation floor contradicts it.
func (cr *cycleRun) recordPlannedNoWorkOutcome() {
	if cr.result.TerminationReason == cycleTerminationTriageClaimFailed &&
		cr.current == PhaseTriage &&
		!cr.o.floorAlreadyCompleted(cr.cs.CompletedPhases) &&
		phasesEndAtTriageWithoutImplementation(cr.result.PhasesRun) {
		cr.result.FinalVerdict = VerdictFAIL
		return
	}
	if cr.result.TerminationReason != CycleTerminationTriageNoWork ||
		cr.current != PhaseTriage ||
		cr.o.floorAlreadyCompleted(cr.cs.CompletedPhases) ||
		!phasesEndAtTriageWithoutImplementation(cr.result.PhasesRun) {
		return
	}
	cr.result.FinalVerdict = VerdictSKIPPED
}
