package core

import (
	"fmt"
	"io"
)

// reviewAndGuard runs the per-phase deliverable review gate + correction ladder,
// the ship-preserve clear, the worktree-leak recovery, and the post-phase
// tree-diff guard (extracted behavior-preserving from RunCycle). dr is a pointer
// because a successful correction re-dispatch UPDATES dr.resp (and dr.phaseReq's
// CorrectionDirective), which recordAndBranch then consumes.
//
// Returns loopAbort + error on a review reject (after corrections), a correction
// dispatch failure, a worktree-leak recovery failure, or a real tree-diff leak;
// loopNext otherwise.
func (cr *cycleRun) reviewAndGuard(next Phase, dr *dispatchResult) (loopAction, error) {
	if phaseErr := cr.prepareForReview(next); phaseErr != nil {
		cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
		cr.recordFailureLearning(next, phaseErr, 1)
		return loopAbort, phaseErr
	}

	if action, err := cr.reviewWithCorrections(next, dr); err != nil {
		return action, err
	}

	return cr.applyPostReviewGuards(next, dr)
}

// prepareForReview establishes the filesystem view a reviewer is allowed to
// approve. Recovery must precede host normalization, and both must finish
// before the initial review or any corrected re-review.
func (cr *cycleRun) prepareForReview(next Phase) error {
	if err := cr.recoverBeforeReview(next); err != nil {
		return err
	}
	cr.o.normalizeBuildWorktree(cr.ctx, next, cr.cs)
	return nil
}

func (cr *cycleRun) postBuildExplanationRefreshEligible(completed Phase) bool {
	return cr.cs.ExplanationDocumentationVersion != 0 && completed != PhaseBuild &&
		cr.o.worktreePhase(completed) && containsString(cr.cs.CompletedPhases, string(PhaseBuild))
}

func (cr *cycleRun) recoverBeforeReview(next Phase) error {
	if !cr.o.leakRecoverablePhase(next) || cr.cs.ActiveWorktree == "" {
		return nil
	}
	if recoverBuildLeak(cr.ctx, cr.req.ProjectRoot, cr.cs.ActiveWorktree, cr.mainDirtyBaseline, cr.o.worktreePhase(next)) {
		return nil
	}
	return fmt.Errorf("phase %s: worktree-leak recovery failed (main tree left unsafe for review and audit)", next)
}

// filterRealLeaks applies the tree-diff guard's classifier chain to the
// leaked set: workspace legitimacy, scout eval materialization, registered
// TTL-fresh mints, and — LAST, loud (ADR-0080 S4) — the cycle-start-adopted
// console lease, which waives EXACT paths only and prints one WARN per
// waiver so a leased leak can never masquerade as a clean phase.
func filterRealLeaks(next Phase, leaked []string, mints, leased map[string]bool, warn io.Writer) (real []string, waived int) {
	for _, p := range leaked {
		if isLegitimateMainTreePath(p) || isScoutEvalMaterialization(next, p) || isActiveMintPhasePath(mints, p) {
			continue
		}
		if leased[p] {
			waived++
			fmt.Fprintf(warn, "[orchestrator] WARN tree-diff: leaked path %q WAIVED by the cycle-start console lease (ADR-0080 S4) — operator-leased, not a clean phase\n", p)
			continue
		}
		real = append(real, p)
	}
	return real, waived
}
