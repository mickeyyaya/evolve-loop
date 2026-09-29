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

// phaseTimingEntry is an alias for the single-source schema in internal/
// phasetiming — defined once there so the orchestrator (sole writer), the
// dossier producer, and the `evolve cycle timing` CLI cannot drift apart.
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

// phaseOutcomeFrom is the one place a canonical agent verdict is recorded
// as-is while anything else (empty, non-canonical, error-path zero response)
// synthesizes FAIL; a synthesized PASS is structurally impossible.
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

// recordPhaseOutcome is the one chokepoint every terminal disposition of a
// dispatched phase funnels through, so PhasesRun, phase-timing.json and
// <phase>-usage.json always reflect what actually ran.
//
// See ADR-0044.
func (o *Orchestrator) recordPhaseOutcome(result *CycleResult, timings *[]phaseTimingEntry, workspace string, out recovery.PhaseOutcome) {
	o.recorder().Record(result, timings, workspace, out)
	// Shadow: grades the abort reason but changes nothing, the floor still aborts.
	//
	// See ADR-0048.
	if out.AbortReason != "" {
		if tier := failuregrade.Grade(out.AbortReason, failuregrade.Evidence{}); tier != failuregrade.TierAbort {
			fmt.Fprintf(os.Stderr, "[graduated-enforcement SHADOW] phase %s abort reason %q would grade as %s (ADR-0048 Slice A; enforce pending)\n", out.Phase, out.AbortReason, tier)
		}
	}
}

func (o *Orchestrator) recordHostEnding(timings *[]phaseTimingEntry, out recovery.PhaseOutcome) {
	o.recorder().RecordEnding(timings, out)
}

// flushPhaseTimings composes and persists this cycle's phase-timing log
// exactly once and returns the composed set; calling it again returns the
// cached set without re-appending.
func (cr *cycleRun) flushPhaseTimings() []phaseTimingEntry {
	if cr.timingsFlushed {
		return cr.timingsComposed
	}
	cr.timingsFlushed = true
	cr.timingsComposed = cr.o.recorder().WritePhaseTimings(cr.cs.WorkspacePath, cr.phaseTimings)
	return cr.timingsComposed
}

// learningGate is the pure pre-recorder decision of recordFailureLearning:
// which of the three silent exits fires, or that learning proceeds. It never
// mutates the request. Order: incomplete → canceled → quota-deferred. A
// cancellation is an operator/runtime stop, not evidence the task or phase
// failed; an all-families quota exhaustion is a deferred resume checkpoint,
// not a failed phase, so both stay out of failure-learning state.
type learningGate int

const (
	gateLearn         learningGate = iota // proceed to the recorder
	gateIncomplete                        // retro itself failed, no error, or a nil State/CycleState/Result/Timings
	gateCanceled                          // ctx.Err() != nil, checked BEFORE the quota sentinel
	gateQuotaDeferred                     // errors.Is(Err, ErrAllFamiliesExhausted)
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

// recordFailureLearning is the chokepoint every failed phase reaches: the
// gate, the carrier, the recorder, the deterministic arms, the retro dispatch
// and its completion.
func (o *Orchestrator) recordFailureLearning(ctx context.Context, fl failureLearningRequest) {
	gate := failureLearningGate(ctx, fl)
	if gate == gateIncomplete || gate == gateCanceled {
		return
	}
	// Preserve a ship dispatch explanation for the coherence floor even when
	// the quota boundary skips failure learning below.
	if fl.Failed == PhaseShip {
		fl.CycleState.ShipFailReasons = []string{fl.Err.Error()}
	}
	if gate == gateQuotaDeferred {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: the dispatch chain ended at a quota wall; skipping failure learning (DEFERRED, resumable)\n")
		return
	}
	summary, todoID, structured := o.recordFailedApproachState(fl)
	// A missing persona doc is a deterministically known configuration absence:
	// learn it deterministically, no LLM dispatch.
	if errors.Is(fl.Err, ErrAgentDocMissing) {
		fmt.Fprintf(os.Stderr, "[orchestrator] failure-learning: %s persona doc missing — known configuration absence, learned deterministically (no retrospective agent dispatched)\n", fl.Failed)
		o.ensureFailureDigest(fl.Cycle, fl.CycleRequest.ProjectRoot, fl.CycleState.WorkspacePath, string(fl.Failed), fl.Err.Error())
		o.learnDeterministically(ctx, fl, summary, structured)
		return
	}
	retroRunner, ok := o.runners[PhaseRetro]
	if !ok {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: no retro runner registered; queued carryover todo only\n")
		o.writeFailureLearningState(ctx, fl.State)
		return
	}
	retroResp, retroErr := o.dispatchRetro(ctx, fl, retroRunner, summary, todoID)
	if retroErr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: retro failed after %s failure: %v\n", fl.Failed, retroErr)
		o.learnDeterministically(ctx, fl, summary, structured)
		return
	}
	if !IsVerdict(retroResp.Verdict) {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: retro returned non-canonical verdict %q after %s failure\n", retroResp.Verdict, fl.Failed)
		o.learnDeterministically(ctx, fl, summary, structured)
		return
	}
	o.completeRetro(ctx, fl, retroResp, todoID)
}

// learnDeterministically is the one fallback tail: the floor, then the
// persist last. It carries no digest — the doc-missing arm writes its digest
// visibly before calling it, and the two retro tails already wrote theirs
// before the runner ran.
func (o *Orchestrator) learnDeterministically(ctx context.Context, fl failureLearningRequest, summary string, structured *phasecontract.FailureBlock) {
	o.writeDeterministicLearning(fl, summary, structured)
	o.writeFailureLearningState(ctx, fl.State)
}

// dispatchRetro runs the inline retrospective: the request, the failure
// digest before the agent (a digest write failure only WARNs, forensics
// plumbing never blocks learning), the retro stamp on the cycle state (not
// restored afterwards), the pre-retro cycle-state write, the observer around
// the run. Decides nothing: the raw response and error go back to the
// coordinator.
func (o *Orchestrator) dispatchRetro(ctx context.Context, fl failureLearningRequest, runner PhaseRunner, summary, todoID string) (PhaseResponse, error) {
	retroReq := fl.retroRequest(summary, todoID)
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

// completeRetro is the retro's durable completion — a divergent twin of
// phaseCompletionRecord.persist, kept and named rather than folded: the
// ledger entry, CompletedPhases, the post-retro cycle-state write, the
// unconditional checkpoint, the verdict, the disposition gate (a PASS retro
// must still deliver a valid disposition.json agreeing with the digest, else
// the completion surfaces a loud gate reason), the retro outcome, and the
// persist last.
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

func (fl failureLearningRequest) retroRequest(summary, todoID string) PhaseRequest {
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
		Cycle:       fl.Cycle,
		ProjectRoot: fl.CycleRequest.ProjectRoot,
		Workspace:   fl.CycleState.WorkspacePath,
		// Even this out-of-band retro keeps the no-main-tree-cwd invariant.
		Worktree:                        fl.CycleState.ActiveWorktree,
		WorktreeBaseSHA:                 fl.CycleState.WorktreeBaseSHA,
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

// recorder lazily builds and caches the live recorder for an Orchestrator
// assembled as a literal. A nil orchestrator gets a fresh bare, no-op
// Recorder on every call, so every path that records or flushes still has
// one writer.
func (o *Orchestrator) recorder() *outcome.Recorder {
	if o == nil {
		return outcome.NewRecorder(time.Now, func(string) string { return "" }, func(int, recovery.PhaseOutcome) {})
	}
	if o.outcome == nil {
		o.outcome = o.wiredRecorder()
	}
	return o.outcome
}

// wiredRecorder is the one construction of the live recorder, shared by
// NewOrchestrator (eager) and recorder() (lazy). Every collaborator is read
// live: the clock through a closure, the Center through an accessor, since
// tests apply WithSignalCenter after construction.
func (o *Orchestrator) wiredRecorder() *outcome.Recorder {
	return outcome.NewRecorder(func() time.Time { return o.now() }, o.phaseArchetype, o.emitPhaseOutcome,
		outcome.WithSignals(func() *signalcenter.Center { return o.signals }))
}
