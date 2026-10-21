package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/outcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/failuregrade"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type phaseTimingEntry = phasetiming.Entry

type failureLearningRequest struct {
	CycleRequest CycleRequest
	Cycle        int
	Failed       Phase
	Err          error
	Attempt      int
	State        *State
	CycleState   *CycleState
	Context      map[string]string
	Env          map[string]string
	Result       *CycleResult
	Timings      *[]phaseTimingEntry
}

func phaseOutcomeFrom(phase Phase, resp PhaseResponse, attempts int, abortReason, startedAt string) recovery.PhaseOutcome {
	verdict := resp.Verdict
	if !IsVerdict(verdict) {
		verdict = VerdictFAIL
	}
	return recovery.PhaseOutcome{
		Phase:         string(phase),
		Verdict:       verdict,
		CostUSD:       resp.CostUSD,
		DurationMS:    resp.DurationMS,
		BootMS:        resp.BootMS,
		StartedAt:     startedAt,
		AttemptCount:  attempts,
		AbortReason:   abortReason,
		ModelSource:   resp.ModelSource,
		ResolvedModel: resp.ResolvedModel,
		Tokens:        resp.Tokens,
		Diagnostics:   resp.Diagnostics,
	}
}

func (o *Orchestrator) recordPhaseOutcome(result *CycleResult, timings *[]phaseTimingEntry, workspace string, out recovery.PhaseOutcome) {
	o.recorder().Record(result, timings, workspace, out)
	if out.AbortReason != "" {
		if tier := failuregrade.Grade(out.AbortReason, failuregrade.Evidence{}); tier != failuregrade.TierAbort {
			fmt.Fprintf(os.Stderr, "[graduated-enforcement SHADOW] phase %s abort reason %q would grade as %s (ADR-0048 Slice A; enforce pending)\n", out.Phase, out.AbortReason, tier)
		}
	}
}

func (o *Orchestrator) recordHostEnding(timings *[]phaseTimingEntry, out recovery.PhaseOutcome) {
	o.recorder().RecordEnding(timings, out)
}

func (cr *cycleRun) flushPhaseTimings() []phaseTimingEntry {
	if cr.timingsFlushed {
		return cr.timingsComposed
	}
	cr.timingsFlushed = true
	cr.timingsComposed = cr.o.recorder().WritePhaseTimings(cr.cs.WorkspacePath, cr.phaseTimings)
	return cr.timingsComposed
}

type learningGate int

const (
	gateLearn learningGate = iota
	gateIncomplete
	gateCanceled
	gateQuotaDeferred
)

func failureLearningGate(ctx context.Context, fl failureLearningRequest) learningGate {
	if fl.Failed == PhaseRetro || fl.Err == nil || fl.State == nil || fl.CycleState == nil || fl.Result == nil || fl.Timings == nil {
		return gateIncomplete
	}
	if ctx.Err() != nil {
		return gateCanceled
	}
	if errors.Is(fl.Err, ErrAllFamiliesExhausted) {
		return gateQuotaDeferred
	}
	return gateLearn
}

func (o *Orchestrator) recordFailureLearning(ctx context.Context, fl failureLearningRequest) {
	gate := failureLearningGate(ctx, fl)
	if gate == gateIncomplete || gate == gateCanceled {
		return
	}
	if fl.Failed == PhaseShip {
		fl.CycleState.ShipFailReasons = []string{fl.Err.Error()}
	}
	if gate == gateQuotaDeferred {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN failure-learning: the dispatch chain ended at a quota wall; skipping failure learning (DEFERRED, resumable)\n")
		return
	}
	summary, todoID, structured := o.recordFailedApproachState(fl)
	if errors.Is(fl.Err, ErrAgentDocMissing) {
		fmt.Fprintf(o.errOut(), "[orchestrator] failure-learning: %s persona doc missing — known configuration absence, learned deterministically (no retrospective agent dispatched)\n", fl.Failed)
		o.ensureFailureDigest(fl.Cycle, fl.CycleRequest.ProjectRoot, fl.CycleState.WorkspacePath, string(fl.Failed), fl.Err.Error())
		o.learnDeterministically(ctx, fl, summary, structured)
		return
	}
	retroRunner, ok := o.runners[PhaseRetro]
	if !ok {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN failure-learning: no retro runner registered; queued carryover todo only\n")
		o.writeFailureLearningState(ctx, fl.State)
		return
	}
	retroResp, retroErr := o.dispatchRetro(ctx, fl, retroRunner, summary, todoID)
	if retroErr != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN failure-learning: retro failed after %s failure: %v\n", fl.Failed, retroErr)
		o.learnDeterministically(ctx, fl, summary, structured)
		return
	}
	if !IsVerdict(retroResp.Verdict) {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN failure-learning: retro returned non-canonical verdict %q after %s failure\n", retroResp.Verdict, fl.Failed)
		o.learnDeterministically(ctx, fl, summary, structured)
		return
	}
	o.completeRetro(ctx, fl, retroResp, todoID)
}

func (o *Orchestrator) learnDeterministically(ctx context.Context, fl failureLearningRequest, summary string, structured *phasecontract.FailureBlock) {
	o.writeDeterministicLearning(fl, summary, structured)
	o.writeFailureLearningState(ctx, fl.State)
}

func (o *Orchestrator) dispatchRetro(ctx context.Context, fl failureLearningRequest, runner PhaseRunner, summary, todoID string) (PhaseResponse, error) {
	retroReq := fl.retroRequest(o, summary, todoID)
	o.ensureFailureDigest(fl.Cycle, retroReq.ProjectRoot, fl.CycleState.WorkspacePath, string(fl.Failed), fl.Err.Error())
	retroStarted := o.now().UTC()
	fl.CycleState.Phase = string(PhaseRetro)
	fl.CycleState.PhaseStartedAt = retroStarted.Format(time.RFC3339)
	fl.CycleState.ActiveAgent = string(PhaseRetro)
	if err := o.storage.WriteCycleState(ctx, *fl.CycleState); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: write cycle-state pre-retro: %v\n", err)
	}
	cancel := o.observer.Start(ctx, string(PhaseRetro), retroReq)
	retroResp, retroErr := runner.Run(ctx, retroReq)
	if cancel != nil {
		cancel()
	}
	return retroResp, retroErr
}

func (o *Orchestrator) completeRetro(ctx context.Context, fl failureLearningRequest, retroResp PhaseResponse, todoID string) {
	if err := o.ledger.Append(ctx, LedgerEntry{
		TS:       o.now().UTC().Format(time.RFC3339),
		Cycle:    fl.Cycle,
		Role:     string(PhaseRetro),
		Kind:     "phase",
		ExitCode: 0,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: retro ledger append: %v\n", err)
	}
	fl.CycleState.CompletedPhases = append(fl.CycleState.CompletedPhases, string(PhaseRetro))
	if err := o.storage.WriteCycleState(ctx, *fl.CycleState); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: write cycle-state post-retro: %v\n", err)
	}
	if PhaseBoundaryCheckpointer != nil {
		if err := PhaseBoundaryCheckpointer(*fl.CycleState, fl.CycleRequest.ProjectRoot, o.now()); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: retro checkpoint failed: %v\n", err)
		}
	}
	fl.Result.FinalVerdict = retroResp.Verdict
	if gateErr := o.finalizeRetroCompletion(fl.CycleState.WorkspacePath); gateErr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: %v\n", gateErr)
		fl.Result.RetroDecision = "failure-learning: " + gateErr.Error()
	} else {
		fl.Result.RetroDecision = "failure-learning: queued " + todoID
	}
	o.recordPhaseOutcome(fl.Result, fl.Timings, fl.CycleState.WorkspacePath, phaseOutcomeFrom(PhaseRetro, retroResp, 1, "", fl.CycleState.PhaseStartedAt))
	o.writeFailureLearningState(ctx, fl.State)
}

func (fl failureLearningRequest) retroRequest(o *Orchestrator, summary, todoID string) PhaseRequest {
	retroCtx := make(map[string]string, len(fl.Context)+5)
	for k, v := range fl.Context {
		retroCtx[k] = v
	}
	retroCtx["previous_verdict"] = VerdictFAIL
	retroCtx["failed_phase"] = string(fl.Failed)
	retroCtx["failure_error"] = fl.Err.Error()
	retroCtx["failure_attempt"] = strconv.Itoa(fl.Attempt)
	retroCtx["failure_summary"] = summary
	retroCtx["next_cycle_todo_id"] = todoID
	req := PhaseRequest{
		Cycle:                           fl.Cycle,
		ProjectRoot:                     fl.CycleRequest.ProjectRoot,
		Workspace:                       fl.CycleState.WorkspacePath,
		Worktree:                        fl.CycleState.ActiveWorktree,
		WorktreeBaseSHA:                 fl.CycleState.WorktreeBaseSHA,
		Fence:                           o.fenceFor(PhaseRetro, *fl.CycleState),
		ExplanationDocumentationVersion: fl.CycleState.ExplanationDocumentationVersion,
		RunID:                           fl.CycleState.RunID,
		GoalHash:                        fl.CycleRequest.GoalHash,
		PreviousPhase:                   string(fl.Failed),
		Env:                             fl.Env,
		Context:                         retroCtx,
	}
	projectBuildExplanation(fl.CycleRequest.ProjectRoot, *fl.CycleState).apply(&req)
	return req
}

func (o *Orchestrator) recorder() *outcome.Recorder {
	if o == nil {
		return outcome.NewRecorder(time.Now, func(string) string { return "" }, func(int, recovery.PhaseOutcome) {})
	}
	if o.outcome == nil {
		o.outcome = o.wiredRecorder()
	}
	return o.outcome
}

func (o *Orchestrator) wiredRecorder() *outcome.Recorder {
	return outcome.NewRecorder(func() time.Time { return o.now() }, o.phaseArchetype, o.emitPhaseOutcome,
		outcome.WithSignals(func() *signalcenter.Center { return o.signals }))
}
