package core

import (
	"errors"
	"fmt"
	"os"
	"time"
)

func (cr *cycleRun) pauseForQuota(next Phase, resp PhaseResponse, attempt int, cause error) error {
	wall := walkedWall(cause)
	phaseErr := fmt.Errorf("phase %s: %w: the dispatch ended at a wall across %d attempts (%v): %s; checkpoint written, next: %s", next, ErrAllFamiliesExhausted, attempt, cause, wall.Clauses(), WallNextStep(wall, cr.cs.WorkspacePath))
	if wall.CountsAsQuotaPause() {
		cr.emitQuotaPaused(next, phaseErr, wall)
	}
	cr.cs.QuotaWall = wall.Ends
	cr.result.QuotaWall, cr.result.WallPhase = wall.Ends, string(next)
	if QuotaBoundaryCheckpointer != nil {
		if cperr := QuotaBoundaryCheckpointer(cr.cs, cr.req.ProjectRoot, cr.o.now()); cperr != nil {
			cr.result.WallWriteErr = cperr.Error()
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN quota-boundary checkpoint write failed: %v (defer still recorded; resume may re-run completed phases)\n", cperr)
		}
	}
	if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
		TS:       cr.o.now().UTC().Format(time.RFC3339),
		Cycle:    cr.cycle,
		Role:     string(next),
		Kind:     "all_families_exhausted",
		ExitCode: 85,
	}); lerr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN all_families_exhausted ledger append: %v\n", lerr)
	}
	cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempt,
		fmt.Sprintf("%s: %s", abortReasonAllFamiliesExhausted, phaseErr.Error()), cr.cs.PhaseStartedAt))
	cr.o.writePhaseFailureDiag(cr.cs.WorkspacePath, string(next), cr.cycle, phaseErr, attempt)
	cr.recordFailureLearning(next, phaseErr, attempt)
	return wrapCycleLevelError(next, phaseErr)
}

func walkedWall(cause error) Wall {
	var walk WalkError
	if !errors.As(cause, &walk) {
		return Wall{}
	}
	return walk.Wall
}

func withMergedWall(err error, wall Wall) error {
	if err == nil || len(wall.Ends) == 0 {
		return err
	}
	return WalkError{Wall: wall, Err: err}
}

func (cr *cycleRun) noteLoginWall(phase Phase, wall Wall) {
	if !wall.HasLogin() {
		return
	}
	e := WallReport{Cycle: cr.cycle, Phase: string(phase), Wall: wall, Workspace: cr.cs.WorkspacePath}.FailEvent("cycleRun.noteLoginWall")
	e.RunID = cr.cs.RunID
	cr.o.signals.Emit(e)
}
