package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
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
	// The cycle's event stream ends here on both roots: system.failure (if the
	// floors attached one) then cycle.sealed; the abnormal path seals from
	// abnormalEpilogue.
	// See ADR-0101.
	cr.emitCycleClose(cr.result, "cycleRun.completeCycle")
	// ADR-0055: emit this completed cycle's closeout dossier; a fleet lane's goes
	// to the pending dir instead of the corpus (dossierDestination). Best-effort — the cycle
	// has already finalized, so a closeout-artifact write error must not fail it
	// (presence is enforced separately by `evolve dossier verify` against the
	// policy floor). Goal text comes from Context["goal"]; falls back to the goal
	// hash so the dossier's required Goal is never blank.
	if derr := writeCycleDossier(cr.o.gitMutationLock, cr.dossierParams(cr.result.FinalVerdict)); derr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d: closeout dossier not written (non-fatal): %v\n", cr.cycle, derr)
	}
	if perr := pruneToolOutputOnPass(cr.cs.WorkspacePath, cr.result.FinalVerdict); perr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d: the raw tool output stays (non-fatal): %v\n", cr.cycle, perr)
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
		cr.recordTriageEnding(VerdictFAIL, cycleTerminationTriageClaimFailed,
			append(triageErrorDiagnostics(cr.phaseTimings), scopeUnansweredDiagnostics(cr.cs.WorkspacePath)...))
		return
	}
	if cr.result.TerminationReason != CycleTerminationTriageNoWork ||
		cr.current != PhaseTriage ||
		cr.o.floorAlreadyCompleted(cr.cs.CompletedPhases) ||
		!phasesEndAtTriageWithoutImplementation(cr.result.PhasesRun) {
		return
	}
	cr.result.FinalVerdict = VerdictSKIPPED
	cr.recordTriageEnding(VerdictSKIPPED, CycleTerminationTriageNoWork, nil)
}

func (cr *cycleRun) recordTriageEnding(verdict, reason string, diags []Diagnostic) {
	cr.o.recordHostEnding(&cr.phaseTimings,
		phaseOutcomeFrom(PhaseTriage, PhaseResponse{Phase: string(PhaseTriage), Verdict: verdict, Diagnostics: diags}, 0, reason, ""))
}

func triageErrorDiagnostics(timings []phaseTimingEntry) []Diagnostic {
	for i := len(timings) - 1; i >= 0; i-- {
		if timings[i].Phase != string(PhaseTriage) {
			continue
		}
		var errs []Diagnostic
		for _, d := range timings[i].Diagnostics {
			if d.Severity == cyclestate.SeverityError {
				errs = append(errs, d)
			}
		}
		return errs
	}
	return nil
}

func scopeUnansweredDiagnostics(workspace string) []Diagnostic {
	owed := committedset.Unanswered(workspace)
	if len(owed) == 0 {
		return nil
	}
	return []Diagnostic{{
		Severity: cyclestate.SeverityError,
		Code:     cyclestate.DiagCodeTriageScopeUnanswered,
		Subject:  strings.Join(owed, ","),
		Message:  "triage committed nothing and left the lane's pinned items unanswered: " + strings.Join(owed, ", "),
	}}
}

// dossierParams is the ONE projection of a cycleRun into the closeout
// dossier's inputs — the normal closeout and the abnormal epilogue differ
// only in the outcome they record. Goal text comes from Context["goal"] and
// falls back to the goal hash so the dossier's required Goal is never blank;
// the root's WithDossierDestination decision becomes the producer's Destination.
func (cr *cycleRun) dossierParams(outcome string) cycleDossierParams {
	goal := cr.req.Context["goal"]
	if goal == "" {
		goal = cr.req.GoalHash
	}
	return cycleDossierParams{
		ProjectRoot:        cr.req.ProjectRoot,
		WorkspacePath:      cr.cs.WorkspacePath,
		Cycle:              cr.cycle,
		Goal:               goal,
		RunID:              cr.cs.RunID,
		Outcome:            outcome,
		SystemFailure:      cr.result.SystemFailure,
		SkippedPhases:      cr.result.SkippedPhases,
		VerdictsNotAdopted: cr.result.VerdictsNotAdopted,
		SpineFailOpens:     cr.result.SpineFailOpens,
		PhaseTimings:       cr.flushPhaseTimings(),
		Destination:        cr.dossierDestination(),
	}
}

func (cr *cycleRun) dossierDestination() DossierDestination {
	if cr.o.dossierDestination == DossierCommitted && fleetMode(cr.req.Env) {
		return DossierPending
	}
	return cr.o.dossierDestination
}
