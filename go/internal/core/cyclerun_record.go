package core

import (
	"fmt"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func (cr *cycleRun) recordAndBranch(next Phase, dr dispatchResult) (loopAction, error) {
	if err := cr.o.ledger.Append(cr.ctx, LedgerEntry{
		TS:       cr.o.now().UTC().Format(time.RFC3339),
		Cycle:    cr.cycle,
		Role:     string(next),
		Kind:     "phase",
		ExitCode: 0,
	}); err != nil {
		lerr := fmt.Errorf("ledger append for %s: %w", next, err)
		cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, lerr.Error(), cr.cs.PhaseStartedAt))
		return loopAbort, lerr
	}

	cr.o.emitPhaseBindings(cr.ctx, cr.cycle, cr.req.ProjectRoot, cr.cs, next, dr.resp.Verdict)

	if next == PhaseBuild {
		repinShipSHAAfterBuild(cr.req.ProjectRoot)

		if reason := buildGraduationCheck(cr.ctx, cr.cs.ActiveWorktree); reason != "" {
			gerr := fmt.Errorf("build graduation guard: %s", reason)
			cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, gerr.Error(), cr.cs.PhaseStartedAt))
			return loopAbort, gerr
		}
	}

	completion := phaseCompletionRecord{
		orchestrator:            cr.o,
		ctx:                     cr.ctx,
		request:                 cr.req,
		cycle:                   cr.cycle,
		phase:                   next,
		response:                dr.resp,
		attempts:                dr.attemptCount,
		state:                   &cr.state,
		cycleState:              &cr.cs,
		result:                  &cr.result,
		timings:                 &cr.phaseTimings,
		checkpoint:              true,
		includeAuditFailReasons: true,
	}
	if err := completion.persist(); err != nil {
		return loopAbort, err
	}
	cr.current = next
	cr.lastVerdict = dr.resp.Verdict

	if cr.current == PhaseAudit && dr.resp.Verdict == VerdictFAIL {
		branch, reason, sysFail := cr.o.decideAfterAuditFail(cr.cs)
		consumeAuditRepairGrant(&cr.cs, reason)
		if sysFail != nil && cr.result.SystemFailure == nil {
			cr.result.SystemFailure = sysFail
		}
		if branch != PhaseRetro {
			if !cr.o.sm.CanTransition(PhaseAudit, branch) {
				return loopAbort, fmt.Errorf("audit→%s not allowed by state machine", branch)
			}
			cr.scheduledNext = branch
		}
	}

	if branch, ok := cr.o.decideExplanationCorrection(&cr.cs, cr.current, dr.resp.Verdict); ok {
		cr.scheduledNext = branch
	}

	if cr.o.successorStrategy(cr.current) == phasespec.BranchingHistory {
		var branch Phase
		var extraEnv map[string]string
		var reason string
		var sysFail *SystemFailureSignal
		cr.cs.FailedAt = cr.state.FailedAt
		if cr.o.cfg.Stage >= config.StageAdvisory {
			cr.routingSeq++
			branch, extraEnv, reason, sysFail = cr.o.decideAfterRetroRouted(cr.ctx, cr.cycle, cr.cs, cr.routingSeq, dr.resp.Verdict, cr.state.FailedAt, router.RouteInput{
				Cfg:            cr.o.cfg,
				Completed:      cr.cs.CompletedPhases,
				Strict:         cr.workflowConfig.StrictAudit,
				Workspace:      cr.cs.WorkspacePath,
				ProjectRoot:    cr.req.ProjectRoot,
				Cycle:          cr.cycle,
				Env:            cr.envSnap,
				Plan:           cr.clampedPlan,
				IntentRequired: cr.cs.IntentRequired,
				PSMASEnabled:   cr.workflowConfig.PSMASEnabled,
			})
		} else {
			branch, extraEnv, reason, sysFail = cr.o.decideAfterRetro(cr.cs, dr.resp.Verdict, cr.state.FailedAt)
		}
		for k, v := range extraEnv {
			cr.envSnap[k] = v
		}
		consumeBookkeepingRegradeGrant(&cr.cs, reason)
		consumeAuditRepairGrant(&cr.cs, reason)
		reason = cr.o.escalateRetroReasonForHistory(cr.req.ProjectRoot, reason, cr.state.FailedAt)
		if gateErr := cr.o.finalizeRetroCompletion(cr.cs.WorkspacePath); gateErr != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN retro: %v\n", gateErr)
			reason = gateErr.Error() + "; " + reason
		}
		cr.result.RetroDecision = reason
		cr.o.carryDefer(cr.cs, reason, &cr.result)
		if sysFail != nil && cr.result.SystemFailure == nil {
			cr.result.SystemFailure = sysFail
		}
		if branch == PhaseEnd {
			return loopBreak, nil
		}
		if !cr.o.sm.CanTransition(PhaseRetro, branch) {
			return loopAbort, fmt.Errorf("retro→%s not allowed by state machine", branch)
		}
		cr.scheduledNext = branch
	}

	if cr.o.successorStrategy(cr.current) == phasespec.BranchingSignal {
		branch := cr.o.decideAfterDebugger(dr.resp)
		cr.o.recordDebuggerDecision(cr.ctx, cr.cycle, cr.cs, dr.resp)
		if branch == PhaseEnd {
			return cr.endAfterDebugger()
		}
		if next, resumed := cr.o.resumeFleetRebaseAfterDebugger(cr.ctx, cr.req.ProjectRoot, cr.cycle, &cr.cs, branch, cr.recoveryDepth, fleetWidthFromEnv(cr.req.Env)); resumed {
			cr.recoveryDepth++
			if next == PhaseEnd {
				return cr.endAfterDebugger()
			}
			branch = next
		}
		if !cr.o.sm.CanTransition(PhaseDebugger, branch) {
			return loopAbort, fmt.Errorf("debugger→%s not allowed by state machine", branch)
		}
		cr.scheduledNext = branch
	}

	return loopNext, nil
}
