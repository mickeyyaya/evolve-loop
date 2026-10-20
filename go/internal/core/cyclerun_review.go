package core

import (
	"fmt"
	"io"
)

func (cr *cycleRun) reviewAndGuard(next Phase, dr *dispatchResult) (loopAction, error) {
	if phaseErr := cr.prepareForReview(next, dr); phaseErr != nil {
		cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
		cr.recordFailureLearning(next, phaseErr, 1)
		return loopAbort, phaseErr
	}

	action, err := cr.reviewWithCorrections(next, dr)
	if err != nil || action == loopBreak {
		return action, err
	}
	if action == loopContinue {
		guard, gerr := cr.applyPostReviewGuards(next, dr)
		if guard == loopNext {
			guard = loopContinue
		}
		return guard, gerr
	}

	return cr.applyPostReviewGuards(next, dr)
}

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

func filterRealLeaks(leaked []string, exempt leakExemptions, warn io.Writer) (real []string, waived int) {
	for _, p := range leaked {
		if isLegitimateMainTreePath(p) {
			continue
		}
		if owner, held := exempt.heldElsewhere(p); held {
			fmt.Fprintf(warn, "[orchestrator] WARN tree-diff: leaked path %q excused: %s holds it, not this lane\n", p, owner)
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
