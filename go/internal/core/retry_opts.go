package core

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/backfill"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/failurediag"
)

type retryOpts struct {
	quotaExhausted       func(attemptExits []int) bool
	backfill             func(phase Phase, err error, attempt, maxAttempts int) (PhaseResponse, bool)
	optionalInfraSkip    func(phase Phase, err error) bool
	postShipObserverSkip func(phase Phase) bool
	shipRecovery         func(phase Phase, err error, resp PhaseResponse, attempts int) bool
}

func (cr *cycleRun) mainDispatchRetryOpts() retryOpts {
	return retryOpts{
		quotaExhausted:       allFamiliesQuotaExhausted,
		backfill:             cr.backfillExhaustedArtifact,
		optionalInfraSkip:    func(p Phase, err error) bool { return cr.o.optionalInfraSkip(p, err) },
		postShipObserverSkip: func(p Phase) bool { return cr.o.postShipObserverSkip(p, cr.cs.Shipped) },
		shipRecovery:         cr.recoverShipError,
	}
}

func (cr *cycleRun) evaluateBatchRetryOpts() retryOpts {
	return retryOpts{
		optionalInfraSkip:    func(p Phase, err error) bool { return cr.o.optionalInfraSkip(p, err) },
		postShipObserverSkip: func(p Phase) bool { return cr.o.postShipObserverSkip(p, cr.cs.Shipped) },
	}
}

func (cr *cycleRun) backfillExhaustedArtifact(next Phase, err error, attempt, maxAttempts int) (PhaseResponse, bool) {
	if !cr.workflowConfig.BackfillEnabled || attempt < maxAttempts || !errors.Is(err, ErrArtifactTimeout) {
		return PhaseResponse{}, false
	}
	artifactPath := backfillArtifactPath(cr.cs.WorkspacePath, string(next))
	ok, berr := backfill.TryExtract(cr.cs.WorkspacePath, string(next), artifactPath, 200)
	if berr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN backfill %s: %v\n", next, berr)
		return PhaseResponse{}, false
	}
	if !ok {
		return PhaseResponse{}, false
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s: ErrArtifactTimeout exhausted; backfilled artifact from stdout.clean.txt; proceeding with WARN verdict\n", next)
	if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
		TS:       cr.o.now().UTC().Format(time.RFC3339),
		Cycle:    cr.cycle,
		Role:     string(next),
		Kind:     "backfill",
		ExitCode: failurediag.ExitCodeArtifactTimeout,
	}); lerr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN backfill ledger append: %v\n", lerr)
	}
	return PhaseResponse{Phase: string(next), Verdict: VerdictWARN, ArtifactsDir: cr.cs.WorkspacePath}, true
}

func (cr *cycleRun) recoverShipError(next Phase, err error, resp PhaseResponse, attempts int) bool {
	raised, ok := AsShipError(err)
	if !ok {
		return false
	}
	cr.preserveWorktree = true
	fleetWidth := fleetWidthFromEnv(cr.req.Env)
	se, declined := cr.carryAuditedChangeOverMovedMain(raised)
	rec, recovering := cr.o.recoverFromShipError(cr.ctx, cr.req.ProjectRoot, cr.cycle, &cr.cs, se, cr.recoveryDepth, fleetWidth)
	cr.reportRecoveryPath(shipRecovery{raised: raised, routed: se, next: rec, declined: declined})
	if !recovering {
		if se != raised {
			cr.pendUnusedCarrier()
		}
		return false
	}
	cr.cs.ShipRecoveryCode = string(se.Code)
	cr.ctxSnap[CtxKeyShipErrorCode] = string(se.Code)
	cr.ctxSnap["ship_error_class"] = string(se.Class)
	cr.ctxSnap["ship_error_stage"] = string(se.Stage)
	cr.ctxSnap["ship_error_debug"] = se.DebugString()
	cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempts,
		fmt.Sprintf("ship error %s: recovering via %s (attempt %d/%d)", se.Code, rec, cr.recoveryDepth+1, shipRecoveryBudget(se.Code, fleetWidth)), cr.cs.PhaseStartedAt))
	cr.recoveryDepth++
	cr.scheduledNext = rec
	cr.current = PhaseShip
	return true
}

func (cr *cycleRun) retryPhaseRunner(phase Phase, req PhaseRequest, opts retryOpts) (PhaseResponse, int, error) {
	maxAttempts := cr.retryConfig.PhaseMaxAttempts
	runner := cr.o.runners[phase]
	var resp PhaseResponse
	var err error
	var attemptExits []int
	for attempt := 1; ; attempt++ {
		req = cr.countDispatch(phase, req)
		obsCancel := cr.o.observer.Start(cr.ctx, string(phase), req)
		resp, err = cr.o.runPhase(cr.ctx, phase, runner, req)
		if obsCancel != nil {
			obsCancel()
		}
		if err == nil && IsVerdict(resp.Verdict) {
			return resp, attempt, nil
		}
		if err != nil {
			attemptExits = append(attemptExits, quotaExit(err))
			if attempt >= maxAttempts || !IsInfraTeardownError(err) {
				if opts.quotaExhausted != nil && opts.quotaExhausted(attemptExits) {
					return resp, attempt, fmt.Errorf("phase %s: %w", phase, quotaWall{err})
				}
				if opts.backfill != nil {
					if backfilled, ok := opts.backfill(phase, err, attempt, maxAttempts); ok {
						return backfilled, attempt, nil
					}
				}
				if opts.optionalInfraSkip != nil && opts.optionalInfraSkip(phase, err) {
					kind, msg, _ := optionalSkipDetails(phase, err)
					return cr.recordPhaseSkip(phase, err, kind, msg), attempt, nil
				}
				if opts.postShipObserverSkip != nil && opts.postShipObserverSkip(phase) {
					return cr.recordPhaseSkip(phase, err, "post_ship_observer_skip", "best-effort observer failed after Ship; skipping"), attempt, nil
				}
				return resp, attempt, err
			}
		} else if attempt >= maxAttempts {
			return resp, attempt, fmt.Errorf("phase %s returned non-canonical verdict %q", phase, resp.Verdict)
		}
		executeRetryBackoff(attempt, cr.retryConfig.RetryBackoffBaseS)
	}
}

func (cr *cycleRun) recordPhaseSkip(phase Phase, cause error, kind, msg string) PhaseResponse {
	fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN %s (%s)\n", msg, kind)
	if err := cr.o.ledger.Append(cr.ctx, LedgerEntry{
		TS: cr.o.now().UTC().Format(time.RFC3339), Cycle: cr.cycle,
		Role: string(phase), Kind: kind, ExitCode: bridgeExitCode(cause),
	}); err != nil {
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN %s ledger append: %v\n", kind, err)
	}
	return PhaseResponse{Phase: string(phase), Verdict: VerdictSKIPPED,
		ArtifactsDir: cr.cs.WorkspacePath, Diagnostics: []Diagnostic{{Severity: "warn", Message: msg}}}
}

func (cr *cycleRun) resumeDispatchRetryOpts() retryOpts {
	opts := cr.mainDispatchRetryOpts()
	opts.backfill = nil
	opts.shipRecovery = nil
	return opts
}
