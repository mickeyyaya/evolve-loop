package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
)

// applyPostReviewGuards runs only after the deliverable review and correction
// ladder approves. It owns explanation refresh, Ship success latching, and the
// final main-tree leak verdict.
func (cr *cycleRun) applyPostReviewGuards(next Phase, dr *dispatchResult) (loopAction, error) {
	// A legitimate post-Build source writer (notably test-amplification) can
	// change the whole-diff digest without changing the material behavior the
	// Builder documented. Refresh that host snapshot after its deliverable is
	// final. If material scope changed, preserve phase ownership by routing back
	// through Build instead of letting the host rewrite the rationale.
	if cr.postBuildExplanationRefreshEligible(next) {
		requiresBuild, err := explanationdocs.RefreshResult(cr.ctx, explanationBinding(cr.req.ProjectRoot, cr.cs))
		if err != nil {
			phaseErr := fmt.Errorf("refresh Build explanation after %s: %w", next, err)
			cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
			cr.recordFailureLearning(next, phaseErr, 1)
			return loopAbort, phaseErr
		}
		if requiresBuild {
			fmt.Fprintf(os.Stderr, "[orchestrator] phase %s changed material scope after Build — routing back to Build for owner-authored explanation\n", next)
			cr.scheduledNext = PhaseBuild
		}
	}

	// cr.cs.Shipped is also the latch read by postShipObserverSkip on the
	// dispatch abort path (a post-ship observer failure degrades to WARN
	// rather than turning a shipped cycle abnormal) and by the outcome label
	// at closeout; it lives on cr.cs so the next persist checkpoints it.
	if latchShippedState(&cr.cs, next, dr.resp.Verdict) {
		// Ship landed AND survived the deliverable review gate above — the
		// worktree is merged, normal exit cleanup applies. Deliberately
		// AFTER the review gate: a review-rejected ship abort must still
		// preserve the worktree for triage.
		cr.preserveWorktree = false
	}

	if dr.treeGuard != nil {
		res := dr.treeGuard.Check(cr.ctx, cr.req.ProjectRoot, dr.beforeDirty)
		if res.SnapshotMissed {
			return cr.abortUncheckedPhase(next, dr)
		} else if !res.OK() {
			// Attempt phase-agnostic binary churn discard for build artifacts
			var relBin string
			if execPath, err := os.Executable(); err == nil {
				if rel, err := filepath.Rel(cr.req.ProjectRoot, execPath); err == nil && !strings.HasPrefix(rel, "..") {
					relBin = filepath.ToSlash(rel)
				}
			}

			_ = discardMainLeak(cr.ctx, cr.req.ProjectRoot, "go/evolve")
			if relBin != "" && relBin != "go/evolve" {
				if isGitignored(cr.ctx, cr.req.ProjectRoot, relBin) {
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN: relBin path %q is gitignored; skipping discardMainLeak to prevent checkout error\n", relBin)
				} else {
					_ = discardMainLeak(cr.ctx, cr.req.ProjectRoot, relBin)
				}
			}

			res2 := dr.treeGuard.Check(cr.ctx, cr.req.ProjectRoot, dr.beforeDirty)
			if res2.SnapshotMissed {
				return cr.abortUncheckedPhase(next, dr)
			}
			if res2.OK() {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff: discarded binary rebuild churn in phase %s; continuing\n", next)
			} else {
				leaked := res2.Leaked
				realLeaks, waived := filterRealLeaks(leaked, cr.leakExemptions(), os.Stderr)
				if len(realLeaks) == 0 {
					if waived > 0 {
						fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff: phase %s continued only because %d leaked path(s) were console-leased (ADR-0080 S4) — not a clean phase\n", next, waived)
					} else {
						fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff: phase %s wrote only legitimate main-tree paths (.evolve/ workspace); continuing\n", next)
					}
					leaked = nil
				} else {
					leaked = realLeaks
				}
				if len(leaked) > 0 {
					phaseErr := fmt.Errorf("tree-diff guard: phase %q wrote to the main tree outside its worktree %q — leaked paths: %v",
						string(next), dr.phaseWorktree, leaked)
					// The build ran, PASSed, and burned tokens before the guard
					// caught its main-tree leak. The abort is correct; erasing
					// the outcome would not be.
					cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
					cr.recordFailureLearning(next, phaseErr, 1)
					evolveBinPath := filepath.Join(cr.req.ProjectRoot, "go/bin/evolve")
					if _, err := os.Stat(evolveBinPath); os.IsNotExist(err) {
						fmt.Fprintf(os.Stderr, "[orchestrator] ABNORMAL: go/bin/evolve absent after cycle abort — trust-kernel guards degraded\n")
					}
					return loopAbort, phaseErr
				}
			}
		}
	}

	return loopNext, nil
}

func (cr *cycleRun) abortUncheckedPhase(next Phase, dr *dispatchResult) (loopAction, error) {
	phaseErr := fmt.Errorf("tree-diff guard: the post-phase main-tree snapshot for %s failed after %d attempts, so the phase's writes cannot be checked", next, snapshotAttempts)
	cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
	cr.recordFailureLearning(next, phaseErr, 1)
	return loopAbort, phaseErr
}
