package core

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func evaluateBatch(plan []string, archetypeOf func(string) string) []string {
	start := -1
	for i, p := range plan {
		if p == string(PhaseBuild) {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil
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
			break
		}
	}
	if len(batch) < 2 {
		return nil
	}
	return batch
}

func (cr *cycleRun) phaseRequestFor(phase Phase) PhaseRequest {
	req := PhaseRequest{
		Cycle:                           cr.cycle,
		ProjectRoot:                     cr.req.ProjectRoot,
		Workspace:                       cr.cs.WorkspacePath,
		Worktree:                        cr.cs.ActiveWorktree,
		WorktreeReadOnly:                cr.o.worktreeReadOnly(phase),
		WorktreeBaseSHA:                 cr.cs.WorktreeBaseSHA,
		ExplanationDocumentationVersion: cr.cs.ExplanationDocumentationVersion,
		RunID:                           cr.cs.RunID,
		GoalHash:                        cr.req.GoalHash,
		PreviousPhase:                   string(cr.current),
		Env:                             cr.envSnap,
		Context:                         cr.ctxSnap,
		Signals:                         dispatchSignals(phase, cr.cs.WorkspacePath, cr.req.ProjectRoot),
		BypassPolicy:                    cr.req.BypassPolicy,
		OperatorDirectives:              cr.directivesSet.Merged,
	}
	req.BuildPlan = readUpstreamBuildPlan(cr.o.cfg.PhaseIO, phase, cr.workflowConfig.PhaseEnables, cr.cs.WorkspacePath)
	projectBuildExplanation(cr.req.ProjectRoot, cr.cs).apply(&req)
	if cr.o.cfg.PhaseIO >= config.StageShadow {
		req.Input = cr.assemblePhaseIO(phase, cr.cs.ActiveWorktree, cr.ctxSnap)
	}
	return req
}

func (cr *cycleRun) dispatchRunnerWithRetry(phase Phase, req PhaseRequest) (PhaseResponse, int, error) {
	return cr.retryPhaseRunner(phase, req, cr.evaluateBatchRetryOpts())
}

func (cr *cycleRun) dispatchEvaluateBatch(batch []Phase) (loopAction, error) {
	cr.cs.PhaseStartedAt = cr.o.now().UTC().Format(time.RFC3339)

	reqs := make([]PhaseRequest, len(batch))
	for i, p := range batch {
		reqs[i] = cr.phaseRequestFor(p)
	}

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
		cr.o.writePhaseFailureDiag(cr.cs.WorkspacePath, string(errPhase), cr.cycle, firstErr, errAttempts)
		cr.recordFailureLearning(errPhase, perr, errAttempts)
		return loopAbort, wrapCycleLevelError(errPhase, perr)
	}
	cr.lastVerdict = batchVerdict
	cr.result.FinalVerdict = batchVerdict
	return loopNext, nil
}

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

func mergeVerdict(acc, v string) string {
	if acc == VerdictFAIL || v == VerdictFAIL {
		return VerdictFAIL
	}
	if acc == VerdictWARN || v == VerdictWARN {
		return VerdictWARN
	}
	return VerdictPASS
}
