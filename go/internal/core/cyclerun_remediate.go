package core

import (
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards/treediff"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

var remediationDenied = map[Phase]bool{
	PhaseAudit:                  true,
	PhaseRetro:                  true,
	PhaseDebugger:               true,
	Phase("adversarial-review"): true,
	Phase("premise-challenge"):  true,
	Phase("plan-review"):        true,
}

func (cr *cycleRun) maybeRemediate(next Phase, dr *dispatchResult) (loopAction, error) {
	if dr.resp.Verdict != VerdictFAIL {
		return loopNext, nil
	}
	wf := cr.o.workflowConfig
	if wf.RemediationRounds <= 0 || !remediableListed(wf.RemediablePhases, next) || next == PhaseBuild {
		return loopNext, nil
	}
	if remediationDenied[next] {
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN remediation: phase %q is configured remediable but is a judgment/control phase — refused (deterministic gates only)\n", next)
		return loopNext, nil
	}
	builder, ok := cr.o.runners[PhaseBuild]
	if !ok {
		return loopNext, nil
	}
	if cr.remediationRounds == nil {
		cr.remediationRounds = map[Phase]int{}
	}
	if cr.remediationRounds[next] >= wf.RemediationRounds {
		return loopNext, nil
	}
	cr.remediationRounds[next]++
	round := cr.remediationRounds[next]

	report := filepath.Join(cr.cs.WorkspacePath, phasecontract.ArtifactFilename(string(next)))
	fmt.Fprintf(cr.o.errOut(), "[orchestrator] remediation: gate %s FAILed — dispatching builder fix round %d/%d (report: %s)\n",
		next, round, wf.RemediationRounds, report)

	cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, "", cr.cs.PhaseStartedAt))

	fixStarted := cr.o.now().UTC().Format("2006-01-02T15:04:05Z07:00")
	breq := dr.phaseReq
	breq.ModelRoutingCLI = ""
	breq.ModelRoutingTier = ""
	breq.Fence = cr.o.fenceFor(PhaseBuild, cr.cs)
	breq.CorrectionDirective = fmt.Sprintf(
		"REMEDIATION round %d: the deterministic gate %q FAILED after your build. "+
			"Read its report at %s and fix EXACTLY the enumerated defects in the worktree — "+
			"typically missing tests for new code. Do not widen scope; do not touch unrelated "+
			"files. The same gate re-runs immediately after you finish.", round, next, report) + correctionRecord(round)
	fixGuard, fixBefore, bresp, berr := cr.snapshotThenRun(PhaseBuild, builder, breq)
	fixAbort := ""
	if berr != nil {
		fixAbort = fmt.Sprintf("remediation fix dispatch: %v", berr)
	}
	cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(Phase("build-remediation"), bresp, 1, fixAbort, fixStarted))
	if berr != nil || bresp.Verdict == VerdictFAIL {
		note := fmt.Sprintf("%s: round %d builder-failed", next, round)
		cr.result.Remediations = append(cr.result.Remediations, note)
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN remediation: builder fix failed (%v, verdict=%s) — gate FAIL stands\n", berr, bresp.Verdict)
		return loopNext, nil
	}
	fixResult := dispatchResult{
		resp: bresp, attemptCount: 1, phaseWorktree: cr.cs.ActiveWorktree,
		runner: builder, phaseReq: breq, treeGuard: fixGuard, beforeDirty: fixBefore, remediation: true,
	}
	if act, reviewErr := cr.reviewAndGuard(PhaseBuild, &fixResult); act == loopAbort {
		note := fmt.Sprintf("%s: round %d voided (Build review rejected: %v)", next, round, reviewErr)
		cr.result.Remediations = append(cr.result.Remediations, note)
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN remediation: %s — gate FAIL stands\n", note)
		return loopNext, nil
	}
	bresp = fixResult.resp
	cr.o.emitPhaseBindings(cr.ctx, cr.cycle, cr.req.ProjectRoot, cr.cs, PhaseBuild, bresp.Verdict)
	if cr.cs.ExplanationDocumentationVersion != 0 {
		handoff := projectBuildExplanation(cr.req.ProjectRoot, cr.cs)
		if handoff.State != BuildExplanationAvailable {
			note := fmt.Sprintf("%s: round %d voided (resealed Build explanation unavailable: %s)", next, round, handoff.Error)
			cr.result.Remediations = append(cr.result.Remediations, note)
			fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN remediation: %s — gate FAIL stands\n", note)
			return loopNext, nil
		}
		handoff.apply(&dr.phaseReq)
	}

	cr.cs.PhaseStartedAt = cr.o.now().UTC().Format("2006-01-02T15:04:05Z07:00")
	greq := dr.phaseReq
	greq.CorrectionDirective = fmt.Sprintf("RE-VERIFY after remediation round %d: run your full check fresh against the current worktree.", round)
	rerunGuard, rerunBefore, resp2, rerr := cr.snapshotThenRun(next, dr.runner, greq)
	if rerr != nil {
		if isQuotaWall(rerr) {
			return loopAbort, cr.pauseForQuota(next, dr.resp, dr.attemptCount+round, rerr)
		}
		note := fmt.Sprintf("%s: round %d re-run dispatch-failed", next, round)
		cr.result.Remediations = append(cr.result.Remediations, note)
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN remediation: gate re-run dispatch failed (%v) — original FAIL stands\n", rerr)
		return loopNext, nil
	}
	dr.resp, dr.treeGuard, dr.beforeDirty = resp2, rerunGuard, rerunBefore
	cr.result.Remediations = append(cr.result.Remediations,
		fmt.Sprintf("%s: round %d -> %s", next, round, resp2.Verdict))
	fmt.Fprintf(cr.o.errOut(), "[orchestrator] remediation: gate %s re-ran -> %s (round %d/%d)\n",
		next, resp2.Verdict, round, wf.RemediationRounds)
	if act, err := cr.reviewAndGuard(next, dr); act == loopAbort || err != nil {
		return act, err
	}
	return loopNext, nil
}

func (cr *cycleRun) snapshotThenRun(phase Phase, runner PhaseRunner, req PhaseRequest) (*treediff.Guard, []string, PhaseResponse, error) {
	guard, before, err := cr.snapshotMainTree(phase)
	if err != nil {
		return nil, nil, PhaseResponse{}, err
	}
	resp, err := cr.observedRun(phase, runner, req)
	return guard, before, resp, err
}

func (cr *cycleRun) observedRun(phase Phase, runner PhaseRunner, req PhaseRequest) (PhaseResponse, error) {
	req = cr.countDispatch(phase, req)
	cancel := cr.o.observer.Start(cr.ctx, string(phase), req)
	resp, err := runner.Run(cr.ctx, req)
	if cancel != nil {
		cancel()
	}
	return resp, err
}

func remediableListed(list []string, p Phase) bool {
	for _, s := range list {
		if s == string(p) {
			return true
		}
	}
	return false
}
