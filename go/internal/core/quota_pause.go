package core

import (
	"fmt"
	"os"
	"time"
)

// pauseForQuota records a resumable resource pause. Both dispatch entrypoints
// preserve the same checkpoint and classification rather than creating a FAIL.
func (cr *cycleRun) pauseForQuota(next Phase, resp PhaseResponse, attempt int) error {
	phaseErr := fmt.Errorf("phase %s: %w: every family in the fallback chain returned exit=85 across %d attempts; checkpoint written — resume with `evolve loop --resume` after quota reset", next, ErrAllFamiliesExhausted, attempt)
	fmt.Fprintf(os.Stderr, "[orchestrator] WARN %v\n", phaseErr)
	if QuotaBoundaryCheckpointer != nil {
		if cperr := QuotaBoundaryCheckpointer(cr.cs, cr.req.ProjectRoot, cr.o.now()); cperr != nil {
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
	// ADR-0044 C1: record the abort reason with the DEFERRED
	// prefix so cyclehealth classifies the cycle DEFERRED.
	cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempt,
		fmt.Sprintf("%s: %s", abortReasonAllFamiliesExhausted, phaseErr.Error()), cr.cs.PhaseStartedAt))
	writePhaseFailureDiag(cr.cs.WorkspacePath, string(next), cr.cycle, phaseErr, attempt, cr.o.now)
	cr.recordFailureLearning(next, phaseErr, attempt)
	return wrapCycleLevelError(next, phaseErr)
}
