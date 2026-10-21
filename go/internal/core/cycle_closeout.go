package core

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func (cr *cycleRun) completeCycle() error {
	cr.recordPlannedNoWorkOutcome()
	preserve, ferr := cr.o.finalizeCycle(cr.ctx, cr.cs, cr.cycle, cr.preCycleHEAD, cr.req.ProjectRoot, &cr.result, &cr.state, cr.phaseTimings)
	if preserve {
		cr.preserveWorktree = true
	}
	if ferr != nil {
		return ferr
	}
	cr.cycleCompletedNormally = true
	cr.emitCycleClose(cr.result, "cycleRun.completeCycle")
	if derr := writeCycleDossier(cr.o.gitMutationLock, cr.dossierParams(cr.result.FinalVerdict)); derr != nil {
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN cycle %d: closeout dossier not written (non-fatal): %v\n", cr.cycle, derr)
	}
	cr.writeShadowRetrospectives()
	if perr := pruneToolOutputOnPass(cr.cs.WorkspacePath, cr.result.FinalVerdict); perr != nil {
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN cycle %d: the raw tool output stays (non-fatal): %v\n", cr.cycle, perr)
	}
	return nil
}

func (cr *cycleRun) recordPlannedNoWorkOutcome() {
	if cr.result.TerminationReason == CycleTerminationTddProtectedRoute && cr.current == PhaseTDD {
		cr.result.FinalVerdict = VerdictFAIL
		cr.o.recordHostEnding(&cr.phaseTimings, phaseOutcomeFrom(PhaseTDD,
			PhaseResponse{Phase: string(PhaseTDD), Verdict: VerdictFAIL, Diagnostics: cr.protectedRouteDiags},
			0, CycleTerminationTddProtectedRoute, ""))
		return
	}
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

func (cr *cycleRun) dossierParams(outcome string) cycleDossierParams {
	goal := cr.req.Context["goal"]
	if goal == "" {
		goal = cr.req.GoalHash
	}
	return cycleDossierParams{
		ProjectRoot:            cr.req.ProjectRoot,
		WorkspacePath:          cr.cs.WorkspacePath,
		Cycle:                  cr.cycle,
		Goal:                   goal,
		RunID:                  cr.cs.RunID,
		Outcome:                outcome,
		SystemFailure:          cr.result.SystemFailure,
		SkippedPhases:          cr.result.SkippedPhases,
		VerdictsNotAdopted:     cr.result.VerdictsNotAdopted,
		SpineFailOpens:         cr.result.SpineFailOpens,
		ExplanationCorrections: cr.cs.ExplanationCorrections,
		PhaseTimings:           cr.flushPhaseTimings(),
		Destination:            cr.dossierDestination(),
	}
}

func (cr *cycleRun) dossierDestination() DossierDestination {
	if cr.o.dossierDestination == DossierCommitted && fleetMode(cr.req.Env) {
		return DossierPending
	}
	return cr.o.dossierDestination
}
