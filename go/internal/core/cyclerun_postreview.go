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
	// dispatch abort path (a post-ship observer failure degrades to WARN rather
	// than turning a shipped cycle abnormal — cycle-574) and by the outcome
	// label at closeout; it lives on cr.cs so the next persist checkpoints it.
	if latchShippedState(&cr.cs, next, dr.resp.Verdict) {
		// Ship landed AND survived the deliverable review gate above — the
		// worktree is merged, normal exit cleanup applies. Deliberately
		// AFTER the review gate: a review-rejected ship abort must still
		// preserve the worktree for triage (ADR-0039 §8 / D10).
		cr.preserveWorktree = false
	}

	// Workstream B: post-phase tree-diff check. Runs BEFORE the ledger
	// append so a leak aborts the cycle without recording the phase as a
	// success. Snapshot failures (pre OR post) degrade silently — the
	// guard is belt-and-suspenders to the OS sandbox, so a transient git
	// read error must never cause a false abort.
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

			// Always discard "go/evolve" and relBin (if set)
			_ = discardMainLeak(cr.ctx, cr.req.ProjectRoot, "go/evolve")
			if relBin != "" && relBin != "go/evolve" {
				if isGitignored(cr.ctx, cr.req.ProjectRoot, relBin) {
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN: relBin path %q is gitignored; skipping discardMainLeak to prevent checkout error\n", relBin)
				} else {
					_ = discardMainLeak(cr.ctx, cr.req.ProjectRoot, relBin)
				}
			}

			// Re-snapshot and check again
			res2 := dr.treeGuard.Check(cr.ctx, cr.req.ProjectRoot, dr.beforeDirty)
			if res2.OK() {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff: discarded binary rebuild churn in phase %s; continuing\n", next)
			} else {
				// Filter the leaked set through isLegitimateMainTreePath for EVERY
				// phase — the same classification recoverBuildLeak applies (R9: one
				// shared vocabulary). Non-worktree phases need it for their
				// .evolve/ workspace writes (R7); worktree phases need it because
				// orchestrator-side gates write their own untracked runtime state
				// (.evolve/contract-gate-breaker.json) into the main tree mid-phase
				// — recovery skips those by design, so a strict guard here turned
				// every contract-gate trip into a false cycle abort (the cycle-274
				// salvage CI regression). PLUS a guard-only second classifier,
				// isScoutEvalMaterialization: scout writes its selected evals to the
				// main tree by contract (materialization.go), which recoverBuildLeak
				// never sees (scout is not a WorktreePhase) so it lives only here
				// (soak-#6 cycle 318→319). PLUS a third guard-only classifier,
				// isActiveMintPhasePath: in fleet mode a CONCURRENT lane's advisor
				// mint persists .evolve/phases/<name>/phase.json into the SHARED
				// tree this lane diffs, charging the mint to an innocent phase
				// (cycle-967 false-abort). The registrar records minted names in
				// the shared mintregistry before persisting, so a registered,
				// TTL-fresh name is mint infrastructure, not a leak; an
				// UNREGISTERED phase-config write still aborts. A registry read
				// error only disables the exemption (guard stays armed — the
				// fail-safe direction). Real escapes stay armed: source files and
				// non-scout/non-eval deliverable paths classify as leaks, and
				// porcelainDirtySet emits both rename sides so a deliverable renamed
				// to a .evolve/evals/ look-alike still aborts via its source path.
				leaked := res2.Leaked
				mints, mintErr := mintregistry.ActiveNames(mintregistry.Path(cr.req.ProjectRoot), time.Now())
				if mintErr != nil {
					// ABNORMAL, not WARN: a corrupt registry is either damage or a
					// deliberate availability attack (a lane-wide exemption outage
					// reproduces the cycle-967 false-abort). Quarantine bounds the
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
					// ADR-0044 C1 — THE cycle-262 path: the build ran, PASSed,
					// and burned tokens before the guard caught its main-tree
					// leak. The abort is correct; erasing the outcome was not.
					cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
					cr.recordFailureLearning(next, phaseErr, 1)
					// After abort, check if go/bin/evolve is absent
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
