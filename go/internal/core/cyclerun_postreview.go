package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/mintregistry"
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

	// The post-phase tree-diff check runs BEFORE the ledger append so a leak
	// aborts the cycle without recording the phase as a success. Snapshot
	// failures (pre OR post) degrade silently — the guard is
	// belt-and-suspenders to the OS sandbox, so a transient git read error
	// must never cause a false abort.
	if dr.treeGuard != nil && !dr.snapshotFailed {
		res := dr.treeGuard.Check(cr.ctx, cr.req.ProjectRoot, dr.beforeDirty)
		if res.SnapshotMissed {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff post-phase snapshot failed for %s (sandbox guard degraded; not aborting)\n", next)
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
			if res2.OK() {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff: discarded binary rebuild churn in phase %s; continuing\n", next)
			} else {
				// isLegitimateMainTreePath, isScoutEvalMaterialization and
				// isActiveMintPhasePath each exempt one class of legitimate
				// main-tree write from the leak guard — a phase's own .evolve/
				// workspace writes, scout's contract-mandated eval writes, and a
				// concurrent fleet lane's registered mint — so real escapes stay
				// armed: source files, non-scout/non-eval deliverable paths, and
				// both sides of a leak-disguising rename still abort.
				leaked := res2.Leaked
				mints, mintErr := mintregistry.ActiveNames(mintregistry.Path(cr.req.ProjectRoot), time.Now())
				if mintErr != nil {
					// ABNORMAL, not WARN: a corrupt registry is either damage or a
					// deliberate availability attack. Quarantine bounds the
					// outage to this one check; the guard stays armed either way.
					fmt.Fprintf(os.Stderr, "[orchestrator] ABNORMAL tree-diff: mint registry unreadable (%v); mint exemption disabled for this check\n", mintErr)
					if _, qErr := mintregistry.QuarantineCorrupt(mintregistry.Path(cr.req.ProjectRoot)); qErr != nil {
						fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff: mint registry quarantine failed: %v\n", qErr)
					}
				} else {
					mints = verifiedActiveMints(cr.req.ProjectRoot, mints)
				}
				realLeaks, waived := filterRealLeaks(next, leaked, mints, cr.consoleLeased, os.Stderr)
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
