package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// resumeExecution owns execution of the validated checkpoint suffix. Bootstrap
// identity and process resources are established by RunCycleFromPhase before
// this component starts.
type resumeExecution struct {
	orchestrator      *Orchestrator
	ctx               context.Context
	request           CycleRequest
	resumePoint       *ResumePoint
	state             State
	cycleState        CycleState
	cycle             int
	startPhase        Phase
	envSnapshot       map[string]string
	contextSnapshot   map[string]string
	initialResult     CycleResult
	preResumeHEAD     string
	mainDirtyBaseline map[string]bool
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
	mainDirtyBaseline := r.mainDirtyBaseline

	// ADR-0044 C1 (deferred-to-C3 debt, now paid): the resume path was a
	// SECOND recording boundary that wrote no timings/sidecars at all —
	// resumed phases were invisible in phase-timing.json. Every terminal
	// disposition below funnels through the same recordPhaseOutcome
	// chokepoint RunCycle uses; the deferred writer flushes on abort too
	// and APPEND-MERGES with the pre-crash entries (writePhaseTimings).
	// Semantic note: PhasesRun now includes aborted-but-DISPATCHED phases on
	// resume too (the chokepoint appends on every terminal path) — same
	// what-actually-ran contract RunCycle adopted in Slice 1; consumers are
	// printing/telemetry only (audited then).
	var phaseTimings []phaseTimingEntry
	completed := false
	// Keep one owner for timing composition across abnormal and normal exits.
	// Recreating cycleRun at either boundary would append the resumed segment
	// twice and make phase-timing.json disagree with the dossier.
	timingOwner := &cycleRun{
		o: o, ctx: context.Background(), req: req, cycle: cycle,
		cs: cs, state: state, result: result,
	}
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
		next, err := cursor.next(o, cs)
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
			// ADR-0044 C1 (cycle-637): a stranded successor on RESUME is a
			// terminal disposition that must funnel through the recording
			// chokepoint — the cycle-635 resume died FAILED_UNEXPLAINED precisely
			// because this bare error escaped it. Record a synthetic outcome
			// carrying the abort_reason so the outcome is FAILED_EXPLAINED.
			noRunnerErr := fmt.Errorf("%w: no runner registered for phase %s", ErrPhaseInvalid, next)
			o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath,
				phaseOutcomeFrom(next, PhaseResponse{}, 0, noRunnerErr.Error(), ""))
			return result, noRunnerErr
		}

		cs.Phase = string(next)
		// Stamp the per-phase wall-clock start here too (mirrors
		// cyclerun_dispatch.go): the resume path is a first-class dispatch
		// surface, so a resumed phase's timing record must carry started_at —
		// a post-crash resume is exactly when latency evidence matters most.
		cs.PhaseStartedAt = o.now().UTC().Format(time.RFC3339)
		cs.ActiveAgent = string(next)
		if next == PhaseAudit {
			// Mirrors cyclerun_dispatch.go (resume-parity): a resumed audit
			// re-dispatch supersedes any prior attempt's diagnosed-FAIL
			// explanation — stale reasons must never mark a later FAIL as
			// diagnosed to the ADR-0072 floor.
			resetFloorFailReason(&cs, next)
			// Resume-parity for the round's verdict artifacts (cycle-1603):
			// same supersession rule as cyclerun_dispatch.go — a resumed
			// re-audit must not replay the previous round's verdict.
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

		// Resume-path parity for the audit-repair brief (review MEDIUM): the
		// budget half was already mirrored below via consumeAuditRepairGrant, but
		// without seeding HERE a cycle that crashed mid-repair burned an attempt
		// and rebuilt BLIND — the exact crash-resilience case the persisted
		// counter exists for. Same state-derived rule as the live loop.
		// NEXT, not current: at this point `current` is the phase that ran in the
		// PREVIOUS iteration (it is reassigned to next only at the bottom of the
		// loop). Using it meant the first TDD dispatch after a resumed repair
		// grant — Retro->TDD, the exact crash-resume case this exists for — saw
		// repairSeededPhase(PhaseRetro)==false and rebuilt BLIND. Every sibling
		// line in this block keys on next for the same reason.
		phaseCtx := seedAuditRepairContext(ctxSnap, next, cs)
		phaseCtx = o.seedDispatchContext(ctx, phaseCtx, next, cs, req.ProjectRoot)
		archiveRepairPrompts(cs, next)
		if next == PhaseAudit {
			cs.AuditRepairActive = false
		}
		phaseReq := PhaseRequest{
			Cycle:       cycle,
			AuditRound:  cs.AuditDispatches,
			ProjectRoot: req.ProjectRoot,
			Workspace:   cs.WorkspacePath,
			// CB.1: the resume path is a first-class dispatch surface and must
			// thread the persisted worktree like the RunCycle loop does — a
			// resumed phase with Worktree="" runs cwd=main-tree (cycle-280 class).
			Worktree:                        cs.ActiveWorktree,
			WorktreeReadOnly:                o.worktreeReadOnly(next),
			WorktreeBaseSHA:                 cs.WorktreeBaseSHA,
			ExplanationDocumentationVersion: cs.ExplanationDocumentationVersion,
			// CB.5: same rule for the persisted run identity (resume reuses
			// the run-record id, so session names stay run-scoped).
			RunID:         cs.RunID,
			GoalHash:      req.GoalHash,
			PreviousPhase: string(cursor.current),
			Env:           envSnap,
			Context:       phaseCtx,
			Signals:       dispatchSignals(next, cs.WorkspacePath, req.ProjectRoot),
		}
		dispatch := &cycleRun{o: o, ctx: ctx, req: req, cs: cs, cycle: cycle, ctxSnap: ctxSnap, retryConfig: o.retryConfig, workflowConfig: o.workflowConfig}
		dispatch.applyDispatchPolicy(next, &phaseReq)
		if next != PhaseBuild {
			projectBuildExplanation(req.ProjectRoot, cs).apply(&phaseReq)
		}
		retryHooks := retryOpts{quotaExhausted: allFamiliesQuotaExhausted, optionalInfraSkip: o.optionalInfraSkip}
		resp, attempts, err := dispatch.retryPhaseRunner(next, phaseReq, retryHooks)
		if errors.Is(err, ErrAllFamiliesExhausted) {
			dispatch.result, dispatch.phaseTimings = result, phaseTimings
			err = dispatch.pauseForQuota(next, resp, attempts)
			result, phaseTimings = dispatch.result, dispatch.phaseTimings
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
		// Resume parity with reviewAndGuard: host normalization must finish
		// before Build's explanation is reviewed and sealed.
		o.normalizeBuildWorktree(ctx, next, cs)
		resp, err = o.reviewResumedDeliverable(ctx, req.ProjectRoot, cycle, cs, next, runner, phaseReq, resp, mainDirtyBaseline)
		if err != nil {
			o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempts, err.Error(), cs.PhaseStartedAt))
			return result, err
		}
		if cs.ExplanationDocumentationVersion != 0 && next != PhaseBuild &&
			o.worktreePhase(next) && containsString(cs.CompletedPhases, string(PhaseBuild)) {
			requiresBuild, refreshErr := explanationdocs.RefreshResult(ctx, explanationBinding(req.ProjectRoot, cs))
			if refreshErr != nil {
				return result, fmt.Errorf("resume refresh Build explanation after %s: %w", next, refreshErr)
			}
			if requiresBuild {
				cursor.schedule(PhaseBuild)
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

		// Resume-path parity for the audit-FAIL disposition (ADR-0093). Without
		// this branch the resume surface falls through to sm.Next(audit, FAIL) =
		// retro, so a resumed cycle could NEVER repair — and since retro is now
		// terminal, the retry the policy table grants would be silently
		// unreachable on exactly the surface that exists for recovery. The live
		// loop's branch is cyclerun_record.go; both call the same primitive.
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

		// History-branch gate (ADR-0058): the branch-entry CONDITION is lockstep
		// with recordAndBranch (both key on successorStrategy == history, which
		// owns the degrade). The branch BODY differs by design — resume takes the
		// deterministic decideAfterRetro, whereas the live loop additionally
		// routes via decideAfterRetroRouted at advisory stage.
		if o.successorStrategy(cursor.current) == phasespec.BranchingHistory {
			cs.FailedAt = state.FailedAt // S4 dossier non-progress counters (additive)
			branch, extraEnv, reason, sysFail := o.decideAfterRetro(cs, resp.Verdict, state.FailedAt)
			for k, v := range extraEnv {
				envSnap[k] = v
			}
			// Resume-path parity for the bookkeeping-regrade once-per-cycle bound
			// (cyclerun_record.go, same recurrence class as the floor-verdict
			// guard above): without consuming the slot HERE, a resumed cycle
			// whose re-audit fails bookkeeping-only again regrades forever
			// (bounded only by the resume safety counter — ~15 LLM dispatches).
			// The next pre-phase WriteCycleState persists the consumed slot.
			consumeBookkeepingRegradeGrant(&cs, reason)
			result.RetroDecision = reason
			// ADR-0072 S4: the Go floor is non-bypassable on the resume path too —
			// a floor category halts + escalates rather than looping as task-level.
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

		// The debugger signal-branch gate (ADR-0058 S3) is intentionally NOT
		// mirrored here. Per the ADR the debugger override is live-loop-only: of
		// the two record/resume override sites, resume duplicates only the retro
		// (history) override. A cycle resumed at debugger therefore terminates via
		// Next(debugger,_)→end rather than re-running decideAfterDebugger — the
		// unchanged pre-ADR behavior. Lifting that to resume is a separate slice,
		// not an S3 byte-identity change.
	}

	if !cursor.reachedEnd {
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
