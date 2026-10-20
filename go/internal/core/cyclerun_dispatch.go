package core

import (
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/failurediag"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards/treediff"
)

func (cr *cycleRun) dispatch(next Phase) (dispatchResult, loopAction, error) {
	runner, ok := cr.o.runners[next]
	if !ok {
		if next.IsValid() || isConfiguredMandatory(cr.o.cfg, string(next)) {
			return dispatchResult{}, loopAbort, fmt.Errorf("%w: no runner registered for phase %s", ErrPhaseInvalid, next)
		}
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN phase %s selected but no runner is registered — skipping (register a runner or remove it from the registry order)\n", next)
		cr.current = next
		return dispatchResult{}, loopContinue, nil
	}

	phaseStarted := cr.o.now().UTC()
	cr.cs.Phase = string(next)
	cr.cs.PhaseStartedAt = phaseStarted.Format(time.RFC3339)
	cr.cs.ActiveAgent = string(next)
	if next == PhaseAudit {
		resetFloorFailReason(&cr.cs, next)
		supersedePreviousAuditRound(&cr.cs)
	}
	if err := cr.o.storage.WriteCycleState(cr.ctx, cr.cs); err != nil {
		return dispatchResult{}, loopAbort, fmt.Errorf("write cycle-state pre-%s: %w", next, err)
	}
	cr.o.emitPhaseDispatched(cr.cycle, cr.cs.RunID, next)
	if next == PhaseRetro {
		cr.o.ensureFailureDigest(cr.cycle, cr.req.ProjectRoot, cr.cs.WorkspacePath, string(cr.current),
			agentGradedFailReason(string(cr.current), cr.cs.WorkspacePath))
	}

	phaseWorktree := cr.cs.ActiveWorktree
	treeGuard, beforeDirty, snapErr := cr.snapshotMainTree(next)
	if snapErr != nil {
		return dispatchResult{}, loopAbort, fmt.Errorf("phase %s: %w", next, snapErr)
	}
	phaseCtx := cr.ctxSnap
	if next == PhaseRetro {
		phaseCtx = make(map[string]string, len(cr.ctxSnap)+1)
		for k, v := range cr.ctxSnap {
			phaseCtx[k] = v
		}
		phaseCtx["previous_verdict"] = cr.lastVerdict
	}
	phaseCtx = seedAuditRepairContext(phaseCtx, next, cr.cs)
	phaseCtx = cr.o.seedDispatchContext(cr.ctx, phaseCtx, next, cr.cs, cr.req.ProjectRoot)
	archiveRepairPrompts(cr.cs, next)
	if next == PhaseAudit {
		cr.cs.AuditRepairActive = false
	}
	if next == PhaseRetro {
		cr.o.clearRetroDecisionInput(&cr.cs)
	}
	phaseReq := PhaseRequest{
		Cycle:                           cr.cycle,
		AuditRound:                      cr.cs.AuditDispatches,
		ProjectRoot:                     cr.req.ProjectRoot,
		Workspace:                       cr.cs.WorkspacePath,
		Worktree:                        phaseWorktree,
		WorktreeBaseSHA:                 cr.cs.WorktreeBaseSHA,
		ExplanationDocumentationVersion: cr.cs.ExplanationDocumentationVersion,
		RunID:                           cr.cs.RunID,
		GoalHash:                        cr.req.GoalHash,
		PreviousPhase:                   string(cr.current),
		Env:                             cr.envSnap,
		Context:                         phaseCtx,
		Signals:                         dispatchSignals(next, cr.cs.WorkspacePath, cr.req.ProjectRoot),
		BypassPolicy:                    cr.req.BypassPolicy,
		OperatorDirectives:              cr.directivesSet.Merged,
		Fence:                           cr.o.fenceFor(next, cr.cs),
	}
	if cr.o.cfg.ModelRouting == config.ModelRoutingAuto && cr.clampedPlan != nil {
		for _, e := range cr.clampedPlan.Entries {
			if e.Phase == string(next) {
				phaseReq.ModelRoutingCLI = e.CLI
				phaseReq.ModelRoutingTier = e.Tier
				break
			}
		}
	}
	cr.applyDispatchPolicy(next, &phaseReq)
	phaseReq.BuildPlan = readUpstreamBuildPlan(cr.o.cfg.PhaseIO, next, cr.workflowConfig.PhaseEnables, cr.cs.WorkspacePath)
	if next != PhaseBuild {
		projectBuildExplanation(cr.req.ProjectRoot, cr.cs).apply(&phaseReq)
	}
	if cr.o.cfg.PhaseIO >= config.StageShadow {
		phaseReq.Input = cr.assemblePhaseIO(next, phaseWorktree, phaseCtx)
	}
	var resp PhaseResponse
	var err error
	shipRecovered := false
	maxAttempts := cr.retryConfig.PhaseMaxAttempts
	var attemptCount int
	var attemptExits []int
	retryHooks := cr.mainDispatchRetryOpts()
	for attempt := 1; ; attempt++ {
		attemptCount = attempt
		phaseReq = cr.countDispatch(next, phaseReq)
		obsCancel := cr.o.observer.Start(cr.ctx, string(next), phaseReq)
		resp, err = cr.o.runPhase(cr.ctx, next, runner, phaseReq)
		if obsCancel != nil {
			obsCancel()
		}
		if err == nil && IsVerdict(resp.Verdict) {
			if resp.Reconciled {
				if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
					TS:       cr.o.now().UTC().Format(time.RFC3339),
					Cycle:    cr.cycle,
					Role:     string(next),
					Kind:     "reconciled_timeout",
					ExitCode: failurediag.ExitCodeArtifactTimeout,
				}); lerr != nil {
					fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN reconciled_timeout ledger append: %v\n", lerr)
				}
			}
			break
		}
		if err != nil {
			attemptExits = append(attemptExits, bridgeExitCode(err))
			if attempt >= maxAttempts || !IsInfraTeardownError(err) {
				if retryHooks.quotaExhausted(attemptExits) {
					return dispatchResult{}, loopAbort, cr.pauseForQuota(next, resp, attempt, err)
				}
				if backfilled, ok := retryHooks.backfill(next, err, attempt, maxAttempts); ok {
					resp = backfilled
					break
				}
				if retryHooks.optionalInfraSkip(next, err) {
					kind, msg, _ := optionalSkipDetails(next, err)
					cr.recordFailureLearning(next, fmt.Errorf("phase %s: %w", next, err), attempt)
					resp = cr.recordPhaseSkip(next, err, kind, msg)
					break
				}
				if retryHooks.shipRecovery(next, err, resp, attemptCount) {
					shipRecovered = true
					break
				}
				if retryHooks.postShipObserverSkip(next) {
					cr.recordFailureLearning(next, fmt.Errorf("phase %s: %w", next, err), attempt)
					resp = cr.recordPhaseSkip(next, err, "post_ship_observer_skip", fmt.Sprintf("phase %s: best-effort observer failed after Ship (%v); skipping", next, err))
					break
				}
				phaseErr := fmt.Errorf("phase %s: %w", next, err)
				cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempt, phaseErr.Error(), cr.cs.PhaseStartedAt))
				cr.o.writePhaseFailureDiag(cr.cs.WorkspacePath, string(next), cr.cycle, err, attempt)
				cr.o.adviseOnUnclassifiedFailure(cr.ctx, cr.cycle, cr.cs.WorkspacePath, cr.req.ProjectRoot, next, err, cr.envSnap)
				cr.recordFailureLearning(next, phaseErr, attempt)
				return dispatchResult{}, loopAbort, wrapCycleLevelError(next, phaseErr)
			}
			fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN phase %s attempt %d/%d hit a transient bridge error or timeout; relaunching (self-heal)\n", next, attempt, maxAttempts)
			if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
				TS:       cr.o.now().UTC().Format(time.RFC3339),
				Cycle:    cr.cycle,
				Role:     string(next),
				Kind:     "phase_retry",
				ExitCode: bridgeExitCode(err),
			}); lerr != nil {
				fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN phase_retry ledger append: %v\n", lerr)
			}
			executeRetryBackoff(attempt, cr.retryConfig.RetryBackoffBaseS)
			continue
		}
		if err == nil && !IsVerdict(resp.Verdict) {
			if attempt >= maxAttempts {
				if degraded, ok := cr.o.nonFloorExhaustionDegrade(next, cr.cs.WorkspacePath, cr.o.floorAlreadyCompleted(cr.cs.CompletedPhases)); ok {
					fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN phase %s exhausted retries with non-canonical verdict %q; non-floor phase degrading to SKIPPED and advancing (%s)\n", next, resp.Verdict, ledgerKindContractExhaustionSkip)
					cr.recordFailureLearning(next, fmt.Errorf("phase %s: non-canonical verdict %q after %d attempts", next, resp.Verdict, attempt), attempt)
					cr.o.noteContractExhaustionSkip(cr.ctx, cr.cs, next)
					resp = degraded
					break
				}
				ferr := fmt.Errorf("phase %s returned non-canonical verdict %q", next, resp.Verdict)
				cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempt, ferr.Error(), cr.cs.PhaseStartedAt))
				cr.o.writePhaseFailureDiag(cr.cs.WorkspacePath, string(next), cr.cycle, ferr, attempt)
				cr.recordFailureLearning(next, ferr, attempt)
				return dispatchResult{}, loopAbort, wrapCycleLevelError(next, ferr)
			}
			fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN phase %s attempt %d/%d returned non-canonical verdict %q; relaunching\n", next, attempt, maxAttempts, resp.Verdict)
			if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
				TS:       cr.o.now().UTC().Format(time.RFC3339),
				Cycle:    cr.cycle,
				Role:     string(next),
				Kind:     "phase_retry",
				ExitCode: 0,
			}); lerr != nil {
				fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN phase_retry ledger append: %v\n", lerr)
			}
			executeRetryBackoff(attempt, cr.retryConfig.RetryBackoffBaseS)
			continue
		}
	}
	if shipRecovered {
		return dispatchResult{}, loopContinue, nil
	}

	return dispatchResult{
		resp:          resp,
		attemptCount:  attemptCount,
		phaseWorktree: phaseWorktree,
		treeGuard:     treeGuard,
		beforeDirty:   beforeDirty,
		runner:        runner,
		phaseReq:      phaseReq,
	}, loopNext, nil
}

func (cr *cycleRun) snapshotMainTree(next Phase) (*treediff.Guard, []string, error) {
	checkout := cr.mainTreeIsCheckout()
	guard := treediff.New(retryingDirtyPaths(cr.o.gitDirtyPaths))
	before, err := cr.firstSnapshot(checkout, guard)
	switch {
	case err == nil:
		return guard, before, nil
	case !checkout:
		return nil, nil, nil
	}
	return nil, nil, fmt.Errorf("%w for %s: %w", errPrePhaseSnapshot, next, err)
}

func (cr *cycleRun) firstSnapshot(checkout bool, guard *treediff.Guard) ([]string, error) {
	if checkout {
		return guard.Snapshot(cr.ctx, cr.req.ProjectRoot)
	}
	return treediff.New(cr.o.gitDirtyPaths).Snapshot(cr.ctx, cr.req.ProjectRoot)
}
