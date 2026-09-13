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

// phaseOutcomeFrom builds the single-source outcome record for one phase
// dispatch (ADR-0044 C1). The verdict reconciliation rule lives HERE and only
// here: a canonical agent verdict is recorded as-is; anything else (empty,
// non-canonical, error-path zero response) synthesizes FAIL. A synthesized
// PASS is structurally impossible — reconciliation only ever describes what
// the agent itself reported.
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

// recordPhaseOutcome is the C1 recording chokepoint (ADR-0044): EVERY
// terminal disposition of a dispatched phase — happy advance AND each abort
// return (exhausted retries, non-canonical verdict, review-gate reject,
// ship-error recovery, worktree-leak recovery failure, tree-diff guard,
// ledger/state persistence failure) — funnels through here exactly once, so
// PhasesRun, phase-timing.json, and <phase>-usage.json always reflect what
// actually ran. cycle-262: the build ran, PASSed, and burned tokens, but the
// tree-guard abort path skipped all three records — the divergence this
// chokepoint makes structurally impossible. Paths where the phase never
// dispatched (no runner registered, pre-phase state-write failure) have no
// outcome to record and stay bare.
func (o *Orchestrator) recordPhaseOutcome(result *CycleResult, timings *[]phaseTimingEntry, workspace string, out recovery.PhaseOutcome) {
	o.recorder().Record(result, timings, workspace, out)
	// ADR-0048 Slice A (SHADOW): grade the abort reason. Observe-only — logs the
	// tier graduated-enforcement WOULD apply; changes nothing (the floor still
	// aborts). Evidence is conservative here (the per-site benign-churn /
	// verified-rebuild predicates are plumbed in the enforce slice), so only the
	// always-correctable classes surface in shadow today.
	if out.AbortReason != "" {
		if tier := failuregrade.Grade(out.AbortReason, failuregrade.Evidence{}); tier != failuregrade.TierAbort {
			fmt.Fprintf(os.Stderr, "[graduated-enforcement SHADOW] phase %s abort reason %q would grade as %s (ADR-0048 Slice A; enforce pending)\n", out.Phase, out.AbortReason, tier)
		}
	}
}

// flushPhaseTimings composes and persists this cycle's phase-timing log EXACTLY
// once and returns the composed set, so every consumer — the durable log, the
// normal-path dossier (completeCycle) and the abort-path dossier
// (abnormalEpilogue) — sees the identical record regardless of which fires
// first. Calling it again returns the cached set without re-appending.
func (cr *cycleRun) flushPhaseTimings() []phaseTimingEntry {
	if cr.timingsFlushed {
		return cr.timingsComposed
	}
	cr.timingsFlushed = true
	cr.timingsComposed = cr.o.recorder().WritePhaseTimings(cr.cs.WorkspacePath, cr.phaseTimings)
	return cr.timingsComposed
}

// learningGate is the PURE pre-recorder decision of recordFailureLearning:
// which of the three silent exits fires, or that learning proceeds. It never
// mutates the request — the ShipFailReasons carrier is the coordinator's,
// written between the gate and the quota return (invariant A).
//
// Order: incomplete → canceled → quota-deferred. Cancellation is an
// operator/runtime stop, not evidence that the task or phase failed: keep the
// active phase intact for the interrupt checkpoint and spend no model call on
// a retrospective that cannot finish under an already-canceled context. An
// all-families quota exhaustion is a DEFERRED resume checkpoint, not a failed
// phase: it stays out of *all* failure-learning state, including the
// FailedRecord and P0 todo the recorder mints — errors.Is, because dispatch
// wraps the sentinel before it reaches this chokepoint.
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

// recordFailureLearning is the chokepoint every failed phase reaches — the
// coordinator of the failure-learning spine (ADR-0103 unit 03b): the gate,
// the carrier, the recorder (invariant D: before the doc-missing arm and the
// runner lookup), the deterministic arms, the retro dispatch and its
// completion. Every stderr line below is verbatim; unit 05 codes them.
func (o *Orchestrator) recordFailureLearning(ctx context.Context, fl failureLearningRequest) {
	gate := failureLearningGate(ctx, fl)
	if gate == gateIncomplete || gate == gateCanceled {
		return
	}
	// Preserve a ship dispatch explanation for the coherence floor even when
	// the quota boundary skips failure learning below (invariant A).
	if fl.Failed == PhaseShip {
		fl.CycleState.ShipFailReasons = []string{fl.Err.Error()}
	}
	if gate == gateQuotaDeferred {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: all CLI families quota-exhausted; skipping failure learning (DEFERRED, resumable)\n")
		return
	}
	summary, todoID, structured := o.recordFailedApproachState(fl)
	// A missing persona doc is a deterministically KNOWN configuration absence
	// (cycle-1551 class; cycles 1619/1620 spent a deep-tier agent on exactly
	// this): learn it deterministically — the FailedRecord and carryover todo
	// above, the failure digest and the lesson artifact — no LLM dispatch.
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

// learnDeterministically is the ONE fallback tail (it was copy-pasted three
// times): the floor, then the persist LAST. It carries no digest — the
// doc-missing arm writes its digest visibly before calling it, and the two
// retro tails already wrote theirs before the runner ran.
func (o *Orchestrator) learnDeterministically(ctx context.Context, fl failureLearningRequest, summary string, structured *phasecontract.FailureBlock) {
	o.writeDeterministicLearning(fl, summary, structured)
	o.writeFailureLearningState(ctx, fl.State)
}

// dispatchRetro runs the inline retrospective: the request, the failure
// digest BEFORE the agent (invariant C — the S1 identity the S2 disposition
// gate cross-checks and an input the agent reads; a digest write failure only
// WARNs, forensics plumbing never blocks learning), the retro stamp on the
// cycle state (NOT restored afterwards — preserved, see the unit doc Q1), the
// pre-retro cycle-state write, the observer around the run. Decides nothing:
// the raw response and error go back to the coordinator.
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

// completeRetro is the retro's durable completion — the divergent twin of
// phaseCompletionRecord.persist, kept and NAMED (the unit doc §2 lists the ten
// divergences; folding them is unit 05's behaviour-change series): the ledger
// entry, CompletedPhases, the post-retro cycle-state write, the unconditional
// checkpoint, the verdict, the disposition gate (S2: a PASS retro must still
// deliver a valid disposition.json agreeing with the S1 digest, else the
// completion surfaces a loud gate reason), the retro outcome, and the persist
// LAST. A failed ledger append is also LEDGER_APPEND_FAILED at the adapter's
// one chokepoint; its line stays until unit 05 codes the nine kept lines.
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
		// CB.1: even this out-of-band retro keeps the no-main-tree-cwd
		// invariant — read-only, but invariants with exceptions aren't structural.
		Worktree:                        fl.CycleState.ActiveWorktree,
		WorktreeBaseSHA:                 fl.CycleState.WorktreeBaseSHA,
		ExplanationDocumentationVersion: fl.CycleState.ExplanationDocumentationVersion,
		// CB.5: and the run identity, for run-scoped session naming.
		RunID:         fl.CycleState.RunID,
		GoalHash:      fl.CycleRequest.GoalHash,
		PreviousPhase: string(fl.Failed),
		Env:           fl.Env,
		Context:       retroCtx,
	}
	projectBuildExplanation(fl.CycleRequest.ProjectRoot, *fl.CycleState).apply(&req)
	return req
}

// recorder returns the unit-01 recorder (ADR-0103). NewOrchestrator builds it
// eagerly with the root's Center. An Orchestrator assembled as a literal —
// the test constructions this package keeps — lazily builds and caches that
// same live recorder on first use. A nil orchestrator (a bare cycleRun with
// no orchestrator at all) gets a fresh bare, no-op Recorder on every call, so
// every path that records or flushes still has ONE writer.
func (o *Orchestrator) recorder() *outcome.Recorder {
	if o == nil {
		return outcome.NewRecorder(time.Now, func(string) string { return "" }, func(int, recovery.PhaseOutcome) {})
	}
	if o.outcome == nil {
		o.outcome = o.wiredRecorder()
	}
	return o.outcome
}

// wiredRecorder is the ONE construction of the live recorder, shared by
// NewOrchestrator (eager) and recorder() (lazy). Every collaborator is read
// live: the clock through a closure (tests swap o.now after construction), the
// archetype lookup as the live method value, the Center through an accessor
// (WithSignalCenter is an option tests apply after construction too); the
// emission keeps module orchestrator.
func (o *Orchestrator) wiredRecorder() *outcome.Recorder {
	return outcome.NewRecorder(func() time.Time { return o.now() }, o.phaseArchetype, o.emitPhaseOutcome,
		outcome.WithSignals(func() *signalcenter.Center { return o.signals }))
}
