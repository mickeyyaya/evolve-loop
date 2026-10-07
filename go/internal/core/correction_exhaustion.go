package core

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

const ledgerKindContractExhaustionSkip = "contract_exhaustion_skip"

func (o *Orchestrator) degradesOnExhaustion(phase Phase, completed []string) bool {
	spec, ok := o.skippableOptional(phase)
	return ok && spec.RoleOrDefault() == phasespec.RoleEvaluate && o.floorAlreadyCompleted(completed)
}

func (o *Orchestrator) degradesRejection(cs CycleState, phase Phase, rr ReviewResult) bool {
	return !rr.DeliverableAbsent && o.degradesOnExhaustion(phase, cs.CompletedPhases)
}

func (o *Orchestrator) reviewInputFor(cs CycleState, phase Phase, projectRoot string, resp PhaseResponse) ReviewInput {
	in := ReviewInputFor(cs, phase, projectRoot)
	in.Response = resp
	in.BreakerExempt = o.degradesOnExhaustion(phase, cs.CompletedPhases)
	return in
}

func (cr *cycleRun) rejectAfterCorrections(next Phase, dr *dispatchResult, rr ReviewResult, corrections int) (loopAction, error) {
	phaseErr := fmt.Errorf("review gate: phase %q deliverable rejected after %d correction(s): %s", next, corrections, rr.Reason)
	if corrections == 0 {
		phaseErr = fmt.Errorf("review gate: phase %q deliverable rejected: %s", next, rr.Reason)
	}
	if cr.o.degradesRejection(cr.cs, next, rr) {
		cr.recordFailureLearning(next, phaseErr, max(corrections, 1))
		dr.resp = cr.o.degradeExhaustedReview(cr.ctx, cr.cs, next, phaseErr)
		return loopNext, nil
	}
	cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
	cr.recordFailureLearning(next, phaseErr, max(corrections, 1))
	return loopAbort, wrapCycleLevelError(next, phaseErr)
}

func (o *Orchestrator) degradeExhaustedReview(ctx context.Context, cs CycleState, phase Phase, cause error) PhaseResponse {
	fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s exhausted its contract corrections; optional evaluate phase degrading to SKIPPED and advancing (%s): %v\n", phase, ledgerKindContractExhaustionSkip, cause)
	o.noteContractExhaustionSkip(ctx, cs, phase)
	return skippedResponse(phase, cs.WorkspacePath)
}

func (o *Orchestrator) noteContractExhaustionSkip(ctx context.Context, cs CycleState, phase Phase) {
	if lerr := o.ledger.Append(ctx, LedgerEntry{
		TS:    o.now().UTC().Format(time.RFC3339),
		Cycle: cs.CycleID,
		Role:  string(phase),
		Kind:  ledgerKindContractExhaustionSkip,
	}); lerr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN %s ledger append: %v\n", ledgerKindContractExhaustionSkip, lerr)
	}
	o.recordReviewSkipped(cs, phase)
}
