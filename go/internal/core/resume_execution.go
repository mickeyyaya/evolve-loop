package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// resumeExecution owns execution of the validated checkpoint suffix. Bootstrap
// identity and process resources are established by RunCycleFromPhase before
// this component starts.
type resumeExecution struct {
	orchestrator    *Orchestrator
	ctx             context.Context
	request         CycleRequest
	resumePoint     *ResumePoint
	state           State
	cycleState      CycleState
	cycle           int
	startPhase      Phase
	envSnapshot     map[string]string
	contextSnapshot map[string]string
	initialResult   CycleResult
	preResumeHEAD   string
	checkout        *checkoutDecision

	closeout *cycleRun
}

func (r *resumeExecution) checkoutDecision() *checkoutDecision {
	if r.checkout == nil {
		r.checkout = &checkoutDecision{}
	}
	return r.checkout
}

func (r *resumeExecution) run() (result CycleResult, retErr error) {
	o := r.orchestrator
	ctx := r.ctx
	req := r.request
	resumePoint := r.resumePoint
	state := r.state
	cs := r.cycleState
	cycle := r.cycle
	startPhase := r.startPhase
	envSnap := r.envSnapshot
	ctxSnap := r.contextSnapshot
	result = r.initialResult
	preResumeHEAD := r.preResumeHEAD

	var phaseTimings []phaseTimingEntry
	completed := false
	// One owner spans both abnormal and normal exits: recreating cycleRun at
	// either boundary would append the resumed segment twice and make
	// phase-timing.json disagree with the dossier.
	timingOwner := &cycleRun{
		o: o, ctx: context.Background(), req: req, cycle: cycle,
		cs: cs, state: state, result: result,
	}
	r.closeout = timingOwner
	defer func() {
		if completed || errors.Is(retErr, ErrAllFamiliesExhausted) || ctx.Err() != nil {
			return
		}
		result.FinalVerdict = VerdictFAIL
		timingOwner.cs, timingOwner.state, timingOwner.result = cs, state, result
		timingOwner.abnormalEpilogue(retErr)
		result = timingOwner.result
	}()
	defer func() {
		// Survey emission is unconditional (a resumed cycle that dispatched
		// nothing still has pre-crash outputs to account); the timings write
		// keeps its emptiness guard. cs.CompletedPhases carries the pre-crash
		// completions plus everything this resume appended.
		emitPhaseOutputsSignal(cs.WorkspacePath, cycle, cs.CompletedPhases,
			phasecontract.NewCatalogResolver(o.catalog.Get))
		if len(phaseTimings) == 0 {
			return
		}
		timingOwner.cs, timingOwner.phaseTimings = cs, phaseTimings
		timingOwner.flushPhaseTimings()
	}()

	cursor := newResumeCursor(startPhase)
	maxIterations := o.maxPhaseIterations
	if maxIterations <= 0 {
		maxIterations = defaultMaxPhaseIterations
	}
	for safety := 0; safety < maxIterations; safety++ {
		next, err := cursor.next(o, cs, req.ProjectRoot)
		if err != nil {
			return result, fmt.Errorf("transition from %s: %w", cursor.current, err)
		}
		if next == PhaseEnd {
			cursor.finish()
			break
		}
		if next == PhaseBuild {
			if err := sealBuildExplanationContext(req.ProjectRoot, cs); err != nil {
				return result, fmt.Errorf("resume seal Build explanation context: %w", err)
			}
		}

		runner, ok := o.runners[next]
		if !ok {
			noRunnerErr := fmt.Errorf("%w: no runner registered for phase %s", ErrPhaseInvalid, next)
			o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath,
				phaseOutcomeFrom(next, PhaseResponse{}, 0, noRunnerErr.Error(), ""))
			return result, noRunnerErr
		}

		cs.Phase = string(next)
		cs.PhaseStartedAt = o.now().UTC().Format(time.RFC3339)
		cs.ActiveAgent = string(next)
		if next == PhaseAudit {
			resetFloorFailReason(&cs, next)
			supersedePreviousAuditRound(&cs)
		}
		if err := o.storage.WriteCycleState(ctx, cs); err != nil {
			return result, fmt.Errorf("write cycle-state pre-%s: %w", next, err)
		}

		if resumePoint.StatePath != "" && ResumeBoundaryCheckpointer != nil {
			if err := ResumeBoundaryCheckpointer(cs, req.ProjectRoot, o.now()); err != nil {
				return result, fmt.Errorf("resume checkpoint before %s: %w", next, err)
			}
		}

		phaseCtx := seedAuditRepairContext(ctxSnap, next, cs)
		phaseCtx = o.seedDispatchContext(ctx, phaseCtx, next, cs, req.ProjectRoot)
		archiveRepairPrompts(cs, next)
		if next == PhaseAudit {
			cs.AuditRepairActive = false
		}
		phaseReq := PhaseRequest{
			Cycle:                           cycle,
			AuditRound:                      cs.AuditDispatches,
			ProjectRoot:                     req.ProjectRoot,
			Workspace:                       cs.WorkspacePath,
			Worktree:                        cs.ActiveWorktree,
			WorktreeBaseSHA:                 cs.WorktreeBaseSHA,
			ExplanationDocumentationVersion: cs.ExplanationDocumentationVersion,
			RunID:                           cs.RunID,
			GoalHash:                        req.GoalHash,
			PreviousPhase:                   string(cursor.current),
			Env:                             envSnap,
			Context:                         phaseCtx,
			Signals:                         dispatchSignals(next, cs.WorkspacePath, req.ProjectRoot),
		}
		phaseReq = o.withWorktreeFence(phaseReq, next, cs)
		dispatch := &cycleRun{o: o, ctx: ctx, req: req, cs: cs, cycle: cycle, ctxSnap: ctxSnap, retryConfig: o.retryConfig, workflowConfig: o.workflowConfig, checkout: r.checkoutDecision()}
		dispatch.applyDispatchPolicy(next, &phaseReq)
		if next != PhaseBuild {
			projectBuildExplanation(req.ProjectRoot, cs).apply(&phaseReq)
		}
		retryHooks := retryOpts{quotaExhausted: allFamiliesQuotaExhausted, optionalInfraSkip: o.optionalInfraSkip}
		phaseBaseline, resp, attempts, err := dispatch.snapshotThenDispatch(next, phaseReq, retryHooks)
		// pauseForQuota records through dispatch, so the resume's own accumulators go in and come back.
		pauseOnQuotaWall := func() {
			dispatch.result, dispatch.phaseTimings = result, phaseTimings
			err = dispatch.pauseForQuota(next, resp, attempts, err)
			result, phaseTimings = dispatch.result, dispatch.phaseTimings
		}
		if errors.Is(err, ErrAllFamiliesExhausted) {
			pauseOnQuotaWall()
			return result, err
		}
		if err != nil {
			phaseErr := fmt.Errorf("phase %s: %w", next, err)
			o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempts, phaseErr.Error(), cs.PhaseStartedAt))
			return result, phaseErr
		}
		if !IsVerdict(resp.Verdict) {
			ferr := fmt.Errorf("phase %s returned non-canonical verdict %q", next, resp.Verdict)
			o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempts, ferr.Error(), cs.PhaseStartedAt))
			return result, ferr
		}
		o.normalizeBuildWorktree(ctx, next, cs, req.ProjectRoot)
		resp, err = o.reviewResumedDeliverable(ctx, req.ProjectRoot, cycle, cs, next, runner, phaseReq, resp, phaseBaseline)
		if errors.Is(err, ErrAllFamiliesExhausted) {
			pauseOnQuotaWall()
			return result, err
		}
		if err != nil {
			o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempts, err.Error(), cs.PhaseStartedAt))
			return result, err
		}
		latchShippedState(&cs, next, resp.Verdict)
		if o.explanationRefreshEligible(cs, next) {
			requiresBuild, refreshErr := explanationdocs.RefreshResult(ctx, explanationBinding(req.ProjectRoot, cs))
			reentry, abortErr := o.routeAfterExplanationRefresh(&cs, next, requiresBuild, refreshErr)
			if abortErr != nil {
				phaseErr := fmt.Errorf("resume refresh Build explanation after %s: %w", next, abortErr)
				o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempts, phaseErr.Error(), cs.PhaseStartedAt))
				return result, phaseErr
			}
			if reentry != "" {
				cursor.schedule(reentry)
			}
		}

		if err := o.ledger.Append(ctx, LedgerEntry{
			TS:       o.now().UTC().Format(time.RFC3339),
			Cycle:    cycle,
			Role:     string(next),
			Kind:     "phase",
			ExitCode: 0,
		}); err != nil {
			lerr := fmt.Errorf("ledger append for %s: %w", next, err)
			o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempts, lerr.Error(), cs.PhaseStartedAt))
			return result, lerr
		}

		o.emitPhaseBindings(ctx, cycle, req.ProjectRoot, cs, next, resp.Verdict)
		completion := phaseCompletionRecord{
			orchestrator: o,
			ctx:          ctx,
			request:      req,
			cycle:        cycle,
			phase:        next,
			response:     resp,
			attempts:     attempts,
			state:        &state,
			cycleState:   &cs,
			result:       &result,
			timings:      &phaseTimings,
		}
		if err := completion.persist(); err != nil {
			return result, err
		}
		cursor.advance(next, resp.Verdict)

		if cursor.current == PhaseAudit && resp.Verdict == VerdictFAIL {
			branch, reason, sysFail := o.decideAfterAuditFail(cs)
			consumeAuditRepairGrant(&cs, reason)
			if sysFail != nil && result.SystemFailure == nil {
				result.SystemFailure = sysFail
			}
			if branch != PhaseRetro {
				if !o.sm.CanTransition(PhaseAudit, branch) {
					return result, fmt.Errorf("audit→%s not allowed by state machine", branch)
				}
				cursor.schedule(branch)
			}
		}

		if o.successorStrategy(cursor.current) == phasespec.BranchingHistory {
			cs.FailedAt = state.FailedAt
			branch, extraEnv, reason, sysFail := o.decideAfterRetro(cs, resp.Verdict, state.FailedAt)
			for k, v := range extraEnv {
				envSnap[k] = v
			}
			consumeBookkeepingRegradeGrant(&cs, reason)
			consumeAuditRepairGrant(&cs, reason)
			reason = o.escalateRetroReasonForHistory(req.ProjectRoot, reason, state.FailedAt)
			if gateErr := o.finalizeRetroCompletion(cs.WorkspacePath); gateErr != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN retro: %v\n", gateErr)
				reason = gateErr.Error() + "; " + reason
			}
			result.RetroDecision = reason
			if sysFail != nil && result.SystemFailure == nil {
				result.SystemFailure = sysFail
			}
			if branch == PhaseEnd {
				cursor.finish()
				break
			}
			if !o.sm.CanTransition(PhaseRetro, branch) {
				return result, fmt.Errorf("retro→%s not allowed by state machine", branch)
			}
			cursor.schedule(branch)
		}
	}

	if !cursor.reachedEnd {
		// timingOwner.ctx is reassigned to the live ctx here, replacing the
		// context.Background() it was constructed with: recordChokepointEscape's
		// failure-learning dispatch skips its retrospective call only when ITS
		// ctx is already cancelled, so it must see the real, possibly-cancelled one.
		timingOwner.ctx, timingOwner.cs, timingOwner.state = ctx, cs, state
		timingOwner.result, timingOwner.phaseTimings = result, phaseTimings
		timingOwner.current = cursor.current
		timingOwner.recordChokepointEscape(fmt.Sprintf(
			"transition-table cycle guard: resume dispatch loop ran %d iterations without reaching PhaseEnd (cursor stalled at phase %q) — a transition cycle prevented termination; ADR-0044 C1 chokepoint escape",
			maxIterations, cursor.current))
		result, phaseTimings = timingOwner.result, timingOwner.phaseTimings
		cs, state = timingOwner.cs, timingOwner.state
		return result, fmt.Errorf("resume iteration limit: %d dispatch iterations without reaching PhaseEnd at %s", maxIterations, cursor.current)
	}
	result.TerminationReason = cursor.termination.reason
	cr := timingOwner
	cr.ctx, cr.cs, cr.state, cr.result = ctx, cs, state, result
	cr.phaseTimings, cr.preCycleHEAD = phaseTimings, preResumeHEAD
	cr.current, cr.lastVerdict = cursor.current, cursor.lastVerdict
	if err := cr.completeCycle(); err != nil {
		result = cr.result
		return result, err
	}
	result = cr.result
	completed = true
	cs.Phase, cs.ActiveAgent = string(PhaseEnd), ""
	cs.FinalVerdict = result.FinalVerdict
	if err := o.storage.WriteCycleState(ctx, cs); err != nil {
		return result, fmt.Errorf("resume terminal state: %w", err)
	}

	return result, nil
}
