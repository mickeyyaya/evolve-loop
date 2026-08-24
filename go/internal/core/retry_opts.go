package core

// retry_opts.go — the ONE registry of phase-retry recovery hooks (cycle-1166,
// evaluate-batch-retry-parity).
//
// Two retry loops existed: the sequential dispatch loop (cyclerun_dispatch.go)
// and the evaluate-batch loop (evaluate_batch.go). They agreed only by hand, so
// each hook the sequential loop grew had to be re-remembered on the batch side —
// and twice was not: optionalInfraSkip and postShipObserverSkip shipped
// sequential-only, so an optional evaluate phase that exhausted infra retries
// aborted the whole batch instead of degrading. Fixing the two misses does not
// fix the CLASS; the next hook diverges the same way.
//
// retryOpts is that class fix: a Strategy value enumerating every recovery hook
// a retry loop may run, with a nil field meaning "this path does not run that
// hook" — divergence made explicit and inspectable instead of implicit and
// invisible. Both paths take their hooks from a constructor here, so a new hook
// is a new FIELD, and a field the batch constructor forgets is visible in one
// place rather than discoverable only by diffing two loops.

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/backfill"
)

// retryOpts carries the per-path recovery hooks consulted when a phase's retry
// budget is exhausted (or the error is not infra-shaped). Every field is a func;
// nil ⇒ that hook is disabled for this path. Adding a hook to a retry loop means
// adding a field here — retry_opts_parity_test.go pins the set by reflection, so
// a hook that skips this struct fails the build's test suite.
type retryOpts struct {
	// backfill attempts to reconstruct a phase's artifact from the driver's
	// stdout after an ErrArtifactTimeout exhaustion. Returns the synthesized
	// response and true when it recovered the phase.
	backfill func(phase Phase, err error, attempt, maxAttempts int) (PhaseResponse, bool)
	// optionalInfraSkip reports whether a catalog-Optional, off-floor phase
	// whose exhaustion is infra-shaped may degrade to WARN and advance.
	optionalInfraSkip func(phase Phase, err error) bool
	// postShipObserverSkip reports whether a best-effort post-ship Control
	// observer's failure may degrade to WARN (shipped state read at call time).
	postShipObserverSkip func(phase Phase) bool
	// shipRecovery routes a structured ShipError to the advisor's recovery
	// chain, returning true when the cycle is recovering rather than aborting.
	// It rewrites the sequential loop's control flow (scheduledNext/current),
	// which the concurrent batch path must never do — see retryPhaseRunner.
	shipRecovery func(phase Phase, err error, resp PhaseResponse, attempts int) bool
}

// mainDispatchRetryOpts is the sequential dispatch loop's hook set — the
// REFERENCE set, wiring every hook. cyclerun_dispatch.go consults these rather
// than calling the underlying predicates directly, so "what can the main loop
// recover from" has exactly one enumeration.
func (cr *cycleRun) mainDispatchRetryOpts() retryOpts {
	return retryOpts{
		backfill:             cr.backfillExhaustedArtifact,
		optionalInfraSkip:    func(p Phase, err error) bool { return cr.o.optionalInfraSkip(p, err) },
		postShipObserverSkip: func(p Phase) bool { return cr.o.postShipObserverSkip(p, cr.shipped) },
		shipRecovery:         cr.recoverShipError,
	}
}

// evaluateBatchRetryOpts is the evaluate-batch path's hook set: the two degrade
// predicates (the parity gap this item was filed for), and deliberately NOT
//   - backfill: it writes into the workspace, and the batch runs its phases
//     concurrently — artifact reconstruction stays on the serial path;
//   - shipRecovery: an evaluate batch never contains ship, and the recovery
//     mutates cycleRun control flow that this path contracts not to touch.
//
// The nil fields ARE the documentation: this path's divergence from the
// reference set is declared, not accidental.
func (cr *cycleRun) evaluateBatchRetryOpts() retryOpts {
	return retryOpts{
		optionalInfraSkip:    func(p Phase, err error) bool { return cr.o.optionalInfraSkip(p, err) },
		postShipObserverSkip: func(p Phase) bool { return cr.o.postShipObserverSkip(p, cr.shipped) },
	}
}

// backfillExhaustedArtifact is the sequential loop's backfill hook: when the
// exhaustion is specifically an ErrArtifactTimeout and backfill is enabled
// (default-on; policy.json can disable it for the cycle), reconstruct the
// phase's artifact from stdout.clean.txt and synthesize a WARN response rather
// than aborting the cycle over a driver that died after doing the work. Returns
// (_, false) when backfill is off, the exhaustion is a different shape, or
// nothing could be extracted — the caller then falls through to the next hook.
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
		ExitCode: 81,
	}); lerr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN backfill ledger append: %v\n", lerr)
	}
	return PhaseResponse{Phase: string(next), Verdict: VerdictWARN, ArtifactsDir: cr.cs.WorkspacePath}, true
}

// recoverShipError is the sequential loop's ship-recovery hook (Component #7):
// ship is a pure executor, so a structured ShipError is resolved by the
// advisor's recovery chain — which records the error, picks the recovery phase
// (re-audit / retry-ship / debugger) and bounds the depth — not by aborting the
// cycle. Returns true when the cycle is RECOVERING (the loop must break and let
// the scheduled recovery phase run); false for a non-ship error, an integrity
// breach, an illegal edge, or exhausted depth, all of which fall through to the
// loud abort. Mutates the loop cursor (scheduledNext/current), which is why only
// the sequential path may wire it.
func (cr *cycleRun) recoverShipError(next Phase, err error, resp PhaseResponse, attempts int) bool {
	se, ok := AsShipError(err)
	if !ok {
		return false
	}
	// Preserve the worktree from the exit cleanup while a ship failure is
	// unresolved (ADR-0039 §8 / D10) — cleared when a later ship succeeds.
	cr.preserveWorktree = true
	fleetWidth := fleetWidthFromEnv(cr.req.Env)
	rec, recovering := cr.o.recoverFromShipError(cr.ctx, cr.cycle, cr.cs, se, cr.recoveryDepth, fleetWidth)
	if !recovering {
		return false
	}
	cr.ctxSnap["ship_error_code"] = string(se.Code)
	cr.ctxSnap["ship_error_class"] = string(se.Class)
	cr.ctxSnap["ship_error_stage"] = string(se.Stage)
	cr.ctxSnap["ship_error_debug"] = se.DebugString()
	// ADR-0044 C1: the failed ship attempt ran and burned budget — record it
	// before routing to recovery. A later successful ship records its own.
	cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempts,
		fmt.Sprintf("ship error %s: recovering via %s (attempt %d/%d)", se.Code, rec, cr.recoveryDepth+1, shipRecoveryBudget(se.Code, fleetWidth)), cr.cs.PhaseStartedAt))
	cr.recoveryDepth++
	cr.scheduledNext = rec
	cr.current = PhaseShip // ship ran (and failed); keep forensics accurate
	return true
}

// retryPhaseRunner is the shared self-heal retry core: run the phase, relaunch on
// an infra teardown (artifact timeout / transient bridge) up to the configured
// cap, and on exhaustion consult opts' recovery hooks before surfacing the
// error. It reads only immutable cr handles (runners, observer, retryConfig) and
// mutates nothing, so it is safe to call concurrently — which is why
// opts.shipRecovery is NOT consulted here: that hook rewrites the sequential
// loop's cursor, so the sequential loop consults it inline where that mutation
// is legal. Returns (resp, attempts, err).
func (cr *cycleRun) retryPhaseRunner(phase Phase, req PhaseRequest, opts retryOpts) (PhaseResponse, int, error) {
	maxAttempts := cr.retryConfig.PhaseMaxAttempts
	runner := cr.o.runners[phase]
	var resp PhaseResponse
	var err error
	for attempt := 1; ; attempt++ {
		obsCancel := cr.o.observer.Start(cr.ctx, string(phase), req)
		resp, err = runner.Run(cr.ctx, req)
		if obsCancel != nil {
			obsCancel()
		}
		if err == nil && IsVerdict(resp.Verdict) {
			return resp, attempt, nil
		}
		if err != nil {
			if attempt >= maxAttempts || !IsInfraTeardownError(err) {
				// Hook order mirrors the sequential loop: reconstruct the
				// artifact if possible, else degrade an optional off-floor
				// phase, else degrade a best-effort post-ship observer.
				if opts.backfill != nil {
					if backfilled, ok := opts.backfill(phase, err, attempt, maxAttempts); ok {
						return backfilled, attempt, nil
					}
				}
				if opts.optionalInfraSkip != nil && opts.optionalInfraSkip(phase, err) {
					_, _, diags := optionalSkipDetails(phase, err)
					return PhaseResponse{Phase: string(phase), Verdict: VerdictWARN, ArtifactsDir: cr.cs.WorkspacePath, Diagnostics: diags}, attempt, nil
				}
				if opts.postShipObserverSkip != nil && opts.postShipObserverSkip(phase) {
					return PhaseResponse{Phase: string(phase), Verdict: VerdictWARN, ArtifactsDir: cr.cs.WorkspacePath}, attempt, nil
				}
				return resp, attempt, err
			}
		} else if attempt >= maxAttempts { // err==nil but non-canonical verdict
			return resp, attempt, fmt.Errorf("phase %s returned non-canonical verdict %q", phase, resp.Verdict)
		}
		executeRetryBackoff(attempt, cr.retryConfig.RetryBackoffBaseS)
	}
}
