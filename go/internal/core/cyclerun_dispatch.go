package core

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/backfill"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards/treediff"
)

// dispatch runs the per-phase runner lookup, the pre-phase cycle-state write +
// tree-diff snapshot, the phase-request build, and the whole inner attempt loop
// (self-heal retries, backfill, optional-infra-skip, ship-error recovery)
// extracted behavior-preserving from RunCycle.
//
// Returns:
//   - loopAbort + error: missing built-in/mandatory runner, pre-phase state
//     write failure, or an exhausted/non-recoverable phase failure.
//   - loopContinue: a non-dispatchable optional USER phase skipped (cr.current
//     advanced), or a ShipError routed to a recovery phase (cr.scheduledNext set).
//   - loopNext + a populated dispatchResult: the phase produced a usable verdict;
//     reviewAndGuard/recordAndBranch consume the dispatchResult.
func (cr *cycleRun) dispatch(next Phase) (dispatchResult, loopAction, error) {
	runner, ok := cr.o.runners[next]
	if !ok {
		// The routing surface (registry order + catalog) can know phases
		// the dispatch surface cannot run (cycle-265: registry-listed
		// `memo` had no .evolve/phases config ⇒ no specrunner; the static
		// order walked into it post-ship and killed a PASSING batch). A
		// non-dispatchable OPTIONAL USER phase is skipped loudly and the
		// walk continues; a missing BUILT-IN or configured-mandatory
		// runner stays fatal — that is a wiring bug, not routing-surface
		// drift (built-ins always have factories; only registry/user
		// phases can be known to routing yet unregistered).
		if next.IsValid() || isConfiguredMandatory(cr.o.cfg, string(next)) {
			return dispatchResult{}, loopAbort, fmt.Errorf("%w: no runner registered for phase %s", ErrPhaseInvalid, next)
		}
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s selected but no runner is registered — skipping (register a runner or remove it from the registry order)\n", next)
		cr.current = next
		return dispatchResult{}, loopContinue, nil
	}

	phaseStarted := cr.o.now().UTC()
	cr.cs.Phase = string(next)
	cr.cs.PhaseStartedAt = phaseStarted.Format(time.RFC3339)
	cr.cs.ActiveAgent = string(next)
	if err := cr.o.storage.WriteCycleState(cr.ctx, cr.cs); err != nil {
		return dispatchResult{}, loopAbort, fmt.Errorf("write cycle-state pre-%s: %w", next, err)
	}

	// CB.1 (concurrency campaign W4): EVERY phase runs with cwd = the cycle
	// worktree — not just the source writers (tdd/build, role-gate-permitted)
	// and audit (issue #9: its verification commands must inspect the
	// builder's pending work). A read-only phase's cwd in the main tree let
	// stray writes and guard misfires land in the live checkout (cycle-280);
	// with the worktree provisioned at cycle start, no phase subprocess
	// touches main at all. cwd is NOT write permission: the write axis
	// (role-gate / tree-diff guard / normalize) still keys off worktreePhase.
	// Empty when provisioning failed — the pre-existing degraded mode.
	phaseWorktree := cr.cs.ActiveWorktree
	// Workstream B: snapshot the main-tree dirty set BEFORE a source-
	// writing phase runs. After it runs we re-snapshot and compare —
	// any newly-dirty MAIN-tree path is a leak that escaped the bridge
	// sandbox (each git worktree is a separate working dir, so its
	// writes don't show up here). The treediff package owns the
	// snapshot/check + SnapshotMissed semantics; the orchestrator just
	// threads it through. Skipped entirely for non-worktree phases.
	var (
		treeGuard      *treediff.Guard
		beforeDirty    []string
		snapshotFailed bool
	)
	if cr.o.gitDirtyPaths != nil {
		treeGuard = treediff.New(cr.o.gitDirtyPaths)
		snap, err := treeGuard.Snapshot(cr.ctx, cr.req.ProjectRoot)
		if err != nil {
			snapshotFailed = true
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff pre-phase snapshot failed for %s: %v (sandbox guard degraded; post-phase leak check skipped)\n", next, err)
		} else {
			beforeDirty = snap
		}
	}
	phaseCtx := cr.ctxSnap
	if next == PhaseRetro {
		phaseCtx = make(map[string]string, len(cr.ctxSnap)+1)
		for k, v := range cr.ctxSnap {
			phaseCtx[k] = v
		}
		phaseCtx["previous_verdict"] = cr.lastVerdict
	}
	phaseReq := PhaseRequest{
		Cycle:         cr.cycle,
		ProjectRoot:   cr.req.ProjectRoot,
		Workspace:     cr.cs.WorkspacePath,
		Worktree:      phaseWorktree,
		RunID:         cr.cs.RunID,
		GoalHash:      cr.req.GoalHash,
		PreviousPhase: string(cr.current),
		Env:           cr.envSnap,
		Context:       phaseCtx,
	}
	// ADR-0050 Phase 3.7: at advisory+, serve the build phase's upstream
	// build-plan via the typed envelope (read once here at the seam) instead of
	// an ad-hoc disk read inside the phase. Off/shadow leave it empty → the phase
	// reads disk as before (byte-identical dispatch).
	phaseReq.BuildPlan = readUpstreamBuildPlan(cr.o.cfg.PhaseIO, next, cr.envSnap, cr.cs.WorkspacePath)
	// ADR-0050 Phase 3.4 (SHADOW) + Phase 3.10 (ENFORCE input). When
	// EVOLVE_PHASE_IO>=shadow, assemble the typed Upstream view from the same
	// upstream this phase is about to receive, compare it to the legacy routing
	// digest, and record any divergence (shadow artifact + ledger). At >=enforce,
	// the same pass also returns the authoritative typed PhaseInput the phase
	// consumes in place of the legacy Context map. At EVOLVE_PHASE_IO=off (default)
	// this is skipped entirely and phaseReq.Input stays the zero value —
	// byte-identical dispatch; below enforce the assembled Input is still zero, so
	// only the flip to enforce changes what a phase observes.
	if cr.o.cfg.PhaseIO >= config.StageShadow {
		phaseReq.Input = cr.assemblePhaseIO(next, phaseWorktree, phaseCtx)
	}
	// Cycle-122 Fix 3 / ADR-0030: attach the per-phase observer
	// goroutine BEFORE runner.Run and cancel it AFTER. noopObserver
	// (default when WithObserver wasn't used) is byte-identical to
	// the pre-fix cycle. Real implementations spawn a stall detector
	// that watches <workspace>/<agent>-stdout.log and emits stall
	// events to <workspace>/<agent>-observer-events.ndjson.
	// Self-heal (Fix D): a bridge ArtifactTimeout (exit=81) is the
	// recoverable "agent produced no artifact within the wait window" case
	// — a stalled launch where a fresh relaunch usually succeeds. Retry the
	// phase a bounded number of times on THAT sentinel only; every other
	// error (and exhaustion of the budget) aborts the cycle as before. A
	// deterministic timeout (e.g. a misconfigured agent) simply fails again
	// and aborts after the cap — at most one wasted retry. The observer is
	// (re)started per attempt so each launch is watched.
	var resp PhaseResponse
	var err error
	// shipRecovered marks that a ShipError was intercepted and routed to a
	// recovery phase instead of aborting; the caller continues the outer loop
	// (skipping verdict/ledger handling for the failed ship).
	shipRecovered := false
	maxAttempts := resolvePhaseMaxAttempts(phaseReq.Env)
	var attemptCount int
	for attempt := 1; ; attempt++ {
		attemptCount = attempt
		obsCancel := cr.o.observer.Start(cr.ctx, string(next), phaseReq)
		resp, err = runner.Run(cr.ctx, phaseReq)
		if obsCancel != nil {
			obsCancel()
		}
		if err == nil && IsVerdict(resp.Verdict) {
			// Self-healing trail: a bridge artifact-wait timeout (exit 81) was
			// reconciled by the runner against a well-formed, gate-passing
			// deliverable, so this phase ships on the agent's own verdict
			// instead of a synthesized FAIL. Record it (mirrors the backfill
			// entry) so the recovery is auditable, never silent.
			if resp.Reconciled {
				if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
					TS:       cr.o.now().UTC().Format(time.RFC3339),
					Cycle:    cr.cycle,
					Role:     string(next),
					Kind:     "reconciled_timeout",
					ExitCode: 81,
				}); lerr != nil {
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN reconciled_timeout ledger append: %v\n", lerr)
				}
			}
			break
		}
		if err != nil {
			if attempt >= maxAttempts || (!errors.Is(err, ErrArtifactTimeout) && !isTransientBridgeError(err)) {
				// Backfill: when exhaustion is specifically due to ErrArtifactTimeout,
				// try to reconstruct the artifact from stdout.clean.txt before aborting.
				// Default-on; disabled only if EVOLVE_BACKFILL_ENABLED is "0" in the
				// per-cycle env SNAPSHOT (ADR-0049 N9: read the snapshot, not live
				// os.Getenv, so a concurrent fleet cycle's env can't flip this cycle's
				// backfill. The snapshot already carries the operator's shell value).
				backfillEnabled := envchain.BoolValue(cr.envSnap["EVOLVE_BACKFILL_ENABLED"], true)
				if attempt >= maxAttempts && errors.Is(err, ErrArtifactTimeout) && backfillEnabled {
					artifactPath := backfillArtifactPath(cr.cs.WorkspacePath, string(next))
					if ok, berr := backfill.TryExtract(cr.cs.WorkspacePath, string(next), artifactPath, 200); berr != nil {
						fmt.Fprintf(os.Stderr, "[orchestrator] WARN backfill %s: %v\n", next, berr)
					} else if ok {
						fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s: ErrArtifactTimeout exhausted; backfilled artifact from stdout.clean.txt; proceeding with WARN verdict\n", next)
						resp = PhaseResponse{Phase: string(next), Verdict: VerdictWARN, ArtifactsDir: cr.cs.WorkspacePath}
						err = nil
						if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
							TS:       cr.o.now().UTC().Format(time.RFC3339),
							Cycle:    cr.cycle,
							Role:     string(next),
							Kind:     "backfill",
							ExitCode: 81,
						}); lerr != nil {
							fmt.Fprintf(os.Stderr, "[orchestrator] WARN backfill ledger append: %v\n", lerr)
						}
						break
					}
				}
				// Optional-phase infra skip (Workstream-D intent on
				// ErrArtifactTimeout; cycle-283): an enrichment phase must not
				// veto completed spine work. When backfill could not reconstruct
				// the artifact, a catalog-Optional, non-floor phase whose
				// exhaustion is infra-shaped degrades to a synthesized WARN and
				// the cycle advances toward audit/ship. The failed attempts stay
				// in failure-learning and the ledger — recovered, never silent.
				if cr.o.optionalInfraSkip(next, err) {
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s: optional phase exhausted infra retries (%v); degrading to WARN and advancing (optional_infra_skip)\n", next, err)
					cr.recordFailureLearning(next, fmt.Errorf("phase %s: %w", next, err), attempt)
					if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
						TS:       cr.o.now().UTC().Format(time.RFC3339),
						Cycle:    cr.cycle,
						Role:     string(next),
						Kind:     "optional_infra_skip",
						ExitCode: bridgeExitCode(err),
					}); lerr != nil {
						fmt.Fprintf(os.Stderr, "[orchestrator] WARN optional_infra_skip ledger append: %v\n", lerr)
					}
					resp = PhaseResponse{Phase: string(next), Verdict: VerdictWARN, ArtifactsDir: cr.cs.WorkspacePath}
					err = nil
					break
				}
				// Ship-error recovery seam (Component #7): ship is a pure
				// executor — a structured ShipError is resolved by the advisor's
				// recovery chain (Strategy + CoR), not by aborting the cycle. The
				// resolver records the error, picks the recovery phase
				// (re-audit / retry-ship / debugger), and bounds the depth.
				// Integrity breaches, an illegal edge, or exhausted depth return
				// (_, false) and fall through to the loud abort below.
				if se, ok := AsShipError(err); ok {
					// Preserve the worktree from the exit cleanup while a
					// ship failure is unresolved (ADR-0039 §8 / D10) —
					// cleared when a later ship attempt succeeds.
					cr.preserveWorktree = true
					if rec, recovering := cr.o.recoverFromShipError(cr.ctx, cr.cycle, cr.cs, se, cr.recoveryDepth); recovering {
						cr.ctxSnap["ship_error_code"] = string(se.Code)
						cr.ctxSnap["ship_error_class"] = string(se.Class)
						cr.ctxSnap["ship_error_stage"] = string(se.Stage)
						cr.ctxSnap["ship_error_debug"] = se.DebugString()
						// ADR-0044 C1: the failed ship attempt ran and burned
						// budget — record it before routing to recovery. A
						// later successful ship records its own outcome.
						cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, resp, attemptCount,
							fmt.Sprintf("ship error %s: recovering via %s", se.Code, rec)))
						cr.recoveryDepth++
						cr.scheduledNext = rec
						cr.current = PhaseShip // ship ran (and failed); keep forensics accurate
						shipRecovered = true
						break
					}
				}
				phaseErr := fmt.Errorf("phase %s: %w", next, err)
				// ADR-0044 C1: record the dispatch outcome BEFORE the
				// failure-learning retro so the timing record stays
				// chronological (failed phase, then retro). No canonical
				// agent verdict exists on this path → synthesized FAIL.
				cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempt, phaseErr.Error()))
				writePhaseFailureDiag(cr.cs.WorkspacePath, string(next), cr.cycle, err, attempt, cr.o.now)
				// ADR-0044 C3: enforce-only, best-effort — classify the
				// unclassified pane via the LLM tail and promote, so the
				// NEXT occurrence is deterministic. Never alters the abort.
				cr.o.adviseOnUnclassifiedFailure(cr.ctx, cr.cycle, cr.cs.WorkspacePath, cr.req.ProjectRoot, next, err, cr.envSnap)
				cr.recordFailureLearning(next, phaseErr, attempt)
				return dispatchResult{}, loopAbort, wrapCycleLevelError(next, phaseErr)
			}
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s attempt %d/%d hit a transient bridge error or timeout; relaunching (self-heal)\n", next, attempt, maxAttempts)
			// Emit structured audit trail for the self-heal retry.
			if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
				TS:       cr.o.now().UTC().Format(time.RFC3339),
				Cycle:    cr.cycle,
				Role:     string(next),
				Kind:     "phase_retry",
				ExitCode: bridgeExitCode(err),
			}); lerr != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase_retry ledger append: %v\n", lerr)
			}
			executeRetryBackoff(attempt, phaseReq.Env)
			continue
		}
		if err == nil && !IsVerdict(resp.Verdict) {
			if attempt >= maxAttempts {
				ferr := fmt.Errorf("phase %s returned non-canonical verdict %q", next, resp.Verdict)
				// ADR-0044 C1: a non-canonical verdict is never recorded
				// raw and never upgraded — phaseOutcomeFrom synthesizes FAIL.
				cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempt, ferr.Error()))
				writePhaseFailureDiag(cr.cs.WorkspacePath, string(next), cr.cycle, ferr, attempt, cr.o.now)
				cr.recordFailureLearning(next, ferr, attempt)
				return dispatchResult{}, loopAbort, wrapCycleLevelError(next, ferr)
			}
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s attempt %d/%d returned non-canonical verdict %q; relaunching\n", next, attempt, maxAttempts, resp.Verdict)
			// Emit structured audit trail for the self-heal retry.
			if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
				TS:       cr.o.now().UTC().Format(time.RFC3339),
				Cycle:    cr.cycle,
				Role:     string(next),
				Kind:     "phase_retry",
				ExitCode: 0,
			}); lerr != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase_retry ledger append: %v\n", lerr)
			}
			executeRetryBackoff(attempt, phaseReq.Env)
			continue
		}
	}
	if shipRecovered {
		return dispatchResult{}, loopContinue, nil // run the recovery phase (scheduledNext) next iteration
	}

	return dispatchResult{
		resp:           resp,
		attemptCount:   attemptCount,
		phaseWorktree:  phaseWorktree,
		treeGuard:      treeGuard,
		beforeDirty:    beforeDirty,
		snapshotFailed: snapshotFailed,
		runner:         runner,
		phaseReq:       phaseReq,
	}, loopNext, nil
}
