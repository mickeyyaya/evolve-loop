package core

import (
	"fmt"
	"io"
)

// dr is a pointer because a successful correction re-dispatch updates dr.resp
// and dr.phaseReq's CorrectionDirective, which recordAndBranch then consumes.
func (cr *cycleRun) reviewAndGuard(next Phase, dr *dispatchResult) (loopAction, error) {
	if phaseErr := cr.prepareForReview(next, dr); phaseErr != nil {
		cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
		cr.recordFailureLearning(next, phaseErr, 1)
		return loopAbort, phaseErr
	}

	if action, err := cr.reviewWithCorrections(next, dr); err != nil {
		return action, err
	}

	return cr.applyPostReviewGuards(next, dr)
}

// Recovery must precede host normalization, and both must finish before the
// initial review or any corrected re-review.
func (cr *cycleRun) prepareForReview(next Phase, dr *dispatchResult) error {
	if err := cr.recoverBeforeReview(next, dr); err != nil {
		return err
	}
	cr.o.normalizeBuildWorktree(cr.ctx, next, cr.cs, cr.req.ProjectRoot)
	return nil
}

func (cr *cycleRun) recoverBeforeReview(next Phase, dr *dispatchResult) error {
	return cr.o.recoverPhaseLeak(cr.ctx, phaseLeakScope{
		projectRoot: cr.req.ProjectRoot,
		cycleState:  cr.cs,
		phase:       next,
		baseline:    cr.recoveryBaselineFor(dr.treeGuard, dr.beforeDirty),
		leased:      cr.consoleLeased,
	})
}

// filterRealLeaks checks the cycle-start console lease last: it waives exact
// paths only, and each waiver prints one WARN so a leased leak can never
// masquerade as a clean phase.
//
// See ADR-0080.
func filterRealLeaks(leaked []string, exempt leakExemptions, warn io.Writer) (real []string, waived int) {
	for _, p := range leaked {
		if isLegitimateMainTreePath(p) || exempt.heldElsewhere(p) {
			continue
		}
		if exempt.leased[p] {
			waived++
			fmt.Fprintf(warn, "[orchestrator] WARN tree-diff: leaked path %q WAIVED by the cycle-start console lease (ADR-0080 S4) — operator-leased, not a clean phase\n", p)
			continue
		}
		real = append(real, p)
	}
	return real, waived
}
