package core

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// evaluate_batch.go — PR2b batch identification (pure). The post-build checking
// phases (archetype "evaluate", excluding the audit verdict-brancher) are
// mutually independent — each reads the same immutable build output and emits an
// independent verdict — so they can run concurrently. evaluateBatch finds that
// parallelizable run in the cycle's planned phase order.

// evaluateBatch returns the contiguous run of parallelizable checking phases:
// the archetype-"evaluate" phases that sit AFTER the build phase and BEFORE
// audit in the plan, excluding audit itself (the sole verdict-BRANCHER, which
// must stay serial). It scans from just after build so a pre-build evaluate
// phase (e.g. bug-reproduction on a bugfix cycle) is never batched. Returns nil
// when there is nothing to parallelize (<2 phases, or build absent) — the caller
// then keeps the sequential path. Pure: no side effects, archetypeOf injected.
func evaluateBatch(plan []string, archetypeOf func(string) string) []string {
	start := -1
	for i, p := range plan {
		if p == string(PhaseBuild) {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil // no build phase in the plan ⇒ no post-build batch
	}
	var batch []string
	for _, p := range plan[start:] {
		if p == string(PhaseAudit) {
			break
		}
		if archetypeOf(p) == "evaluate" {
			batch = append(batch, p)
			continue
		}
		if len(batch) > 0 {
			break // the contiguous evaluate run ended
		}
	}
	if len(batch) < 2 {
		return nil
	}
	return batch
}

// phaseRequestFor builds one evaluate-batch phase's PhaseRequest, mirroring
// dispatch's assembly (the BuildPlan + PhaseIO envelope). Called SERIALLY before
// the concurrent run so the (artifact-writing) PhaseIO assemble never races.
func (cr *cycleRun) phaseRequestFor(phase Phase) PhaseRequest {
	req := PhaseRequest{
		Cycle:              cr.cycle,
		ProjectRoot:        cr.req.ProjectRoot,
		Workspace:          cr.cs.WorkspacePath,
		Worktree:           cr.cs.ActiveWorktree,
		RunID:              cr.cs.RunID,
		GoalHash:           cr.req.GoalHash,
		PreviousPhase:      string(cr.current),
		Env:                cr.envSnap,
		Context:            cr.ctxSnap,
		BypassPolicy:       cr.req.BypassPolicy,
		OperatorDirectives: cr.directivesSet.Merged,
	}
	req.BuildPlan = readUpstreamBuildPlan(cr.o.cfg.PhaseIO, phase, cr.workflowConfig.PhaseEnables, cr.cs.WorkspacePath)
	if cr.o.cfg.PhaseIO >= config.StageShadow {
		req.Input = cr.assemblePhaseIO(phase, cr.cs.ActiveWorktree, cr.ctxSnap)
	}
	return req
}

// dispatchRunnerWithRetry runs ONE phase's runner with the self-heal retry loop
// (ArtifactTimeout / transient-bridge relaunch), in isolation — it reads only
// immutable cr handles (runners, observer, retryConfig) and mutates nothing, so
// it is safe to call concurrently. Returns (resp, attempts, err).
func (cr *cycleRun) dispatchRunnerWithRetry(phase Phase, req PhaseRequest) (PhaseResponse, int, error) {
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
			if attempt >= maxAttempts || (!errors.Is(err, ErrArtifactTimeout) && !isTransientBridgeError(err)) {
				// Dispatch parity with the sequential loop (cyclerun_dispatch.go):
				// a catalog-Optional off-floor phase whose exhaustion is
				// infra-shaped, or a best-effort post-ship Control observer,
				// degrades to WARN+advance instead of aborting the whole batch.
				// Both predicates are pure reads (o.cfg/o.catalog) — safe in this
				// concurrent path. Keeps `ship ⇒ build ∧ audit ∧ tdd` intact:
				// mandatory/floor phases never match, so their errors still
				// propagate. The ledger/failure-learning side effects the
				// sequential path emits are recorded by the batch merge, not here
				// (this helper mutates nothing, per its concurrency contract).
				if cr.o.optionalInfraSkip(phase, err) || cr.o.postShipObserverSkip(phase, cr.shipped) {
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

// dispatchEvaluateBatch runs the parallelizable post-build checking phases
// CONCURRENTLY (ParallelEvaluate=enforce), then folds their outcomes in a single
// SERIALIZED merge. Concurrency is bounded by cfg.ParallelEvaluateConcurrency.
//
// Safety: only runner.Run (the minutes-long LLM slice) runs concurrently; every
// shared-state mutation — recordPhaseOutcome (the ADR-0044 C1 chokepoint),
// CompletedPhases, ledger, cycle-state — happens in the single-goroutine merge,
// so there are no races. Verdict merge is weakest-link (FAIL>WARN>PASS). A hard
// dispatch error is all-or-nothing: every phase's outcome is still recorded
// (C1-complete) and the cycle aborts on the first error.
//
// v1 fidelity gap (documented; closed before the enforce flip): the
// deliverable-correction ladder and the tree-diff leak guard do NOT run for
// batched phases — acceptable because evaluate phases are read-only and the
// feature ships DORMANT (StageOff default), activated only after a shadow soak.
func (cr *cycleRun) dispatchEvaluateBatch(batch []Phase) (loopAction, error) {
	cr.cs.PhaseStartedAt = cr.o.now().UTC().Format(time.RFC3339)

	// 1. SERIAL: build every request first (PhaseIO assemble must not race).
	reqs := make([]PhaseRequest, len(batch))
	for i, p := range batch {
		reqs[i] = cr.phaseRequestFor(p)
	}

	// 2. CONCURRENT: runner.Run + self-heal retry per phase. No cr mutation.
	type res struct {
		resp     PhaseResponse
		attempts int
		err      error
	}
	out := make([]res, len(batch))
	conc := cr.o.cfg.ParallelEvaluateConcurrency
	if conc < 1 {
		conc = 1
	}
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup
	for i := range batch {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			resp, attempts, err := cr.dispatchRunnerWithRetry(batch[i], reqs[i])
			out[i] = res{resp, attempts, err}
		}(i)
	}
	wg.Wait()

	// 3. SERIAL MERGE: record every outcome (C1-complete), then abort if any errored.
	batchVerdict := VerdictPASS
	var firstErr error
	var errPhase Phase
	var errAttempts int
	for i, p := range batch {
		r := out[i]
		reason := ""
		if r.err != nil {
			reason = fmt.Errorf("phase %s: %w", p, r.err).Error()
			if firstErr == nil {
				firstErr, errPhase, errAttempts = r.err, p, r.attempts
			}
		}
		cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(p, r.resp, r.attempts, reason, cr.cs.PhaseStartedAt))
		cr.cs.CompletedPhases = append(cr.cs.CompletedPhases, string(p))
		if r.err == nil {
			if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{TS: cr.o.now().UTC().Format(time.RFC3339), Cycle: cr.cycle, Role: string(p), Kind: "phase", ExitCode: 0}); lerr != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN evaluate-batch ledger append %s: %v\n", p, lerr)
			}
			batchVerdict = mergeVerdict(batchVerdict, r.resp.Verdict)
		}
		cr.current = p
	}
	cr.cs.Phase = string(cr.current)
	if werr := cr.o.storage.WriteCycleState(cr.ctx, cr.cs); werr != nil {
		return loopAbort, fmt.Errorf("write cycle-state post-evaluate-batch: %w", werr)
	}
	if firstErr != nil {
		perr := fmt.Errorf("phase %s: %w", errPhase, firstErr)
		writePhaseFailureDiag(cr.cs.WorkspacePath, string(errPhase), cr.cycle, firstErr, errAttempts, cr.o.now)
		cr.recordFailureLearning(errPhase, perr, errAttempts)
		return loopAbort, wrapCycleLevelError(errPhase, perr)
	}
	cr.lastVerdict = batchVerdict
	cr.result.FinalVerdict = batchVerdict
	return loopNext, nil
}

// planRunOrder is the ordered phase run-set for this cycle: the clamped router
// plan's Run entries (the phases that WILL run), or cfg.Order under the static
// spine. The source evaluateBatch reads, so a skipped phase is never batched.
func (cr *cycleRun) planRunOrder() []string {
	if cr.clampedPlan == nil {
		return cr.o.cfg.Order
	}
	out := make([]string, 0, len(cr.clampedPlan.Entries))
	for _, e := range cr.clampedPlan.Entries {
		if e.Run {
			out = append(out, e.Phase)
		}
	}
	return out
}

// evaluateBatchAt returns the parallelizable checking batch ONLY when `next` is
// its first phase (so the loop batches the whole run in one iteration and never
// re-enters mid-run); otherwise nil → the caller keeps the sequential path.
func (cr *cycleRun) evaluateBatchAt(next Phase) []Phase {
	grp := evaluateBatch(cr.planRunOrder(), cr.o.phaseArchetype)
	if len(grp) < 2 || grp[0] != string(next) {
		return nil
	}
	out := make([]Phase, len(grp))
	for i, p := range grp {
		out[i] = Phase(p)
	}
	return out
}

// mergeVerdict folds a phase verdict into the running batch verdict by
// weakest-link precedence: FAIL dominates WARN dominates PASS.
func mergeVerdict(acc, v string) string {
	if acc == VerdictFAIL || v == VerdictFAIL {
		return VerdictFAIL
	}
	if acc == VerdictWARN || v == VerdictWARN {
		return VerdictWARN
	}
	return VerdictPASS
}
