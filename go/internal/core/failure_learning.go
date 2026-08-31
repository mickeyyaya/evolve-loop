package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/contextfill"
	"github.com/mickeyyaya/evolve-loop/go/internal/faillearn"
	"github.com/mickeyyaya/evolve-loop/go/internal/failuregrade"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

// phaseTimingEntry is an alias for the single-source schema in internal/
// phasetiming — defined once there so the orchestrator (sole writer), the
// dossier producer, and the `evolve cycle timing` CLI cannot drift apart.
type phaseTimingEntry = phasetiming.Entry

type phaseUsageSidecar struct {
	Phase        string  `json:"phase"`
	CostUSD      float64 `json:"cost_usd"`
	DurationMS   int64   `json:"duration_ms"`
	AttemptCount int     `json:"attempt_count"`
	Verdict      string  `json:"verdict"`
	// StartedAt/EndedAt/Archetype mirror phaseTimingEntry (ADR-0044 C1).
	StartedAt string `json:"started_at,omitempty"`
	EndedAt   string `json:"ended_at,omitempty"`
	Archetype string `json:"archetype,omitempty"`
	// AbortReason mirrors phaseTimingEntry.AbortReason (ADR-0044 C1).
	AbortReason string `json:"abort_reason,omitempty"`
	// Tokens (S4) mirrors phaseTimingEntry.Tokens — the terminal attempt's
	// token usage, beside CostUSD. Legacy sidecars without it parse to zero.
	Tokens TokenUsage `json:"tokens,omitempty"`
}

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
	}
}

// contextFillFor derives the terminal attempt's context-window occupancy for one
// phase outcome. ResolvedModel already IS the tier the attempt ran at (phases/
// runner sets it from tieredRes.Tier), so it is the honest lookup key; anything
// that is not a canonical tier yields a zero window and contextfill's
// ErrInvalidWindow, which degrades to (0, false). Unknown fill is recorded as
// ABSENT (both fields omitempty), never as a guessed window, and the error never
// propagates — a phase's timing record must not be lost over a missing tier.
func contextFillFor(out recovery.PhaseOutcome) (ratio float64, hot bool) {
	ratio, err := contextfill.FillRatio(out.Tokens, contextfill.WindowSizeForTier(out.ResolvedModel))
	if err != nil {
		return 0, false
	}
	return ratio, contextfill.IsHot(ratio)
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
	// EndedAt and Archetype are stamped HERE — the single chokepoint owns the
	// end-of-dispatch clock reading and the phase classification, so every
	// terminal path records them consistently without each call site re-reading
	// the clock (drift) or re-deriving the taxonomy (duplication).
	out.EndedAt = o.now().UTC().Format(time.RFC3339)
	out.Archetype = o.phaseArchetype(out.Phase)
	result.PhasesRun = append(result.PhasesRun, Phase(out.Phase))
	// Context fill is derived HERE for the same reason EndedAt/Archetype are: the
	// single chokepoint owns the projection, so every terminal path records it.
	fillRatio, windowHot := contextFillFor(out)
	*timings = append(*timings, phaseTimingEntry{
		Phase:         out.Phase,
		DurationMS:    out.DurationMS,
		BootMS:        out.BootMS,
		Verdict:       out.Verdict,
		CostUSD:       out.CostUSD,
		StartedAt:     out.StartedAt,
		EndedAt:       out.EndedAt,
		Archetype:     out.Archetype,
		AttemptCount:  out.AttemptCount,
		AbortReason:   out.AbortReason,
		ModelSource:   out.ModelSource,
		ResolvedModel: out.ResolvedModel,
		Tokens:        out.Tokens,

		ContextFillRatio: fillRatio,
		ContextWindowHot: windowHot,
	})
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
	// Empty workspace ⇒ no sidecar: filepath.Join("", f) is CWD-relative and
	// leaked <phase>-usage.json into go/cmd/evolve during `go test` (the C1
	// abort-path recording made previously-silent test cycles write). The
	// in-memory record above still stands.
	if workspace == "" {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN: empty workspace — skipping %s-usage.json sidecar (in-memory record kept)\n", out.Phase)
		return
	}
	sidecar := phaseUsageSidecar{
		Phase:        out.Phase,
		CostUSD:      out.CostUSD,
		DurationMS:   out.DurationMS,
		AttemptCount: out.AttemptCount,
		Verdict:      out.Verdict,
		StartedAt:    out.StartedAt,
		EndedAt:      out.EndedAt,
		Archetype:    out.Archetype,
		AbortReason:  out.AbortReason,
		Tokens:       out.Tokens,
	}
	data, err := json.MarshalIndent(sidecar, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN: failed to marshal usage sidecar for %s: %v\n", out.Phase, err)
		return
	}
	path := filepath.Join(workspace, fmt.Sprintf("%s-usage.json", out.Phase))
	if werr := os.WriteFile(path, data, 0o644); werr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN: failed to write usage sidecar for %s to %s: %v\n", out.Phase, path, werr)
	}
}

// writePhaseTimings atomically persists phase-timing.json — shared by
// RunCycle's and RunCycleFromPhase's deferred writers (ADR-0044 C1: one
// record format, every execution path). APPEND-MERGE semantics: entries
// already on disk (a crashed earlier attempt, the pre-resume phases) are
// preserved and the new entries appended — the timing file is a LOG of real
// dispatches, so a phase appearing twice (failed attempt + resumed attempt)
// is reality, not duplication. A fresh cycle workspace has no existing file
// ⇒ byte-identical to the pre-merge behavior. Best-effort: failures WARN,
// never mask the cycle outcome.
func writePhaseTimings(workspace string, timings []phaseTimingEntry) {
	// Same CWD-relative leak guard as the usage sidecar (recordPhaseOutcome).
	if workspace == "" {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN: empty workspace — skipping phase-timing.json write\n")
		return
	}
	timingPath := phasetiming.Path(workspace)
	if prev, rerr := os.ReadFile(timingPath); rerr == nil {
		var existing []phaseTimingEntry
		if jerr := json.Unmarshal(prev, &existing); jerr == nil && len(existing) > 0 {
			timings = append(existing, timings...)
		}
	}
	data, merr := json.Marshal(timings)
	if merr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase-timing marshal: %v\n", merr)
		return
	}
	tmp := timingPath + ".tmp"
	if werr := os.WriteFile(tmp, data, 0o644); werr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase-timing write: %v\n", werr)
		return
	}
	if rerr := os.Rename(tmp, timingPath); rerr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase-timing rename: %v\n", rerr)
	}
}

// phaseFailureDiag is the structured diagnostic written to <phase>-failure-diag.json
// when a mandatory phase aborts after exhausting all retry attempts.
type phaseFailureDiag struct {
	Phase           string `json:"phase"`
	Cycle           int    `json:"cycle"`
	ErrorMessage    string `json:"error_message"`
	DeliveryFailure string `json:"delivery_failure"`
	ExitCode        int    `json:"exit_code"`
	AttemptCount    int    `json:"attempt_count"`
	Timestamp       string `json:"timestamp"`
}

// DeliveryFailureCause returns the classified prompt-delivery failure reason,
// or an empty string when the error is not an evidenced delivery failure.
func DeliveryFailureCause(err error) string {
	if !errors.Is(err, ErrArtifactTimeout) {
		return ""
	}
	_, reason, ok := strings.Cut(err.Error(), `reason="`)
	if !ok {
		return ""
	}
	reason, _, ok = strings.Cut(reason, `"`)
	if !ok || !strings.Contains(reason, "submit_wedged") {
		return ""
	}
	return reason
}

// writePhaseFailureDiag writes a structured diagnostic file to
// <workspace>/<phase>-failure-diag.json. Best-effort: failures are logged to
// stderr but never mask the original error.
func writePhaseFailureDiag(workspace, phase string, cycle int, phaseErr error, attempts int, now func() time.Time) {
	exitCode := 1
	var exitErr *exec.ExitError
	if errors.Is(phaseErr, ErrArtifactTimeout) {
		exitCode = 81
	} else if errors.As(phaseErr, &exitErr) {
		exitCode = exitErr.ExitCode()
	}
	diag := phaseFailureDiag{
		Phase:           phase,
		Cycle:           cycle,
		ErrorMessage:    phaseErr.Error(),
		DeliveryFailure: DeliveryFailureCause(phaseErr),
		ExitCode:        exitCode,
		AttemptCount:    attempts,
		Timestamp:       now().UTC().Format(time.RFC3339),
	}
	data, merr := json.Marshal(diag)
	if merr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-diag marshal: %v\n", merr)
		return
	}
	path := filepath.Join(workspace, phase+"-failure-diag.json")
	tmp := path + ".tmp"
	if werr := os.WriteFile(tmp, data, 0o644); werr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-diag write: %v\n", werr)
		return
	}
	if rerr := os.Rename(tmp, path); rerr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-diag rename: %v\n", rerr)
	}
}

// recordFailedApproachState persists the learn-from-failure STATE for a failed
// phase — the FailedRecord appended to state.FailedAt, a deduped P0 carryover
// todo, and the adopted structured failure block — and returns the summary, todo
// id, and structured block for callers that continue with retro. It does NOT run
// retro: that is the caller's concern. Single-sourced (never_duplicate) between
// the error path (recordFailureLearning, which additionally force-runs retro to
// capture the lesson before an aborted cycle ends) and the success path (a FLOOR
// phase returning a FAIL verdict with err==nil via recordFloorVerdictFailure —
// there the cycle still routes FAIL→retro through the normal state machine, so an
// inline retro would be a duplicate). Callers guarantee fl.State/CycleState are
// non-nil.
func (o *Orchestrator) recordFailedApproachState(fl failureLearningRequest) (summary, todoID string, structured *phasecontract.FailureBlock) {
	summary = failureLearningSummary(fl.Cycle, fl.Failed, fl.Err)
	todoID = fmt.Sprintf("cycle-%d-failed-%s", fl.Cycle, fl.Failed)
	now := o.now().UTC()
	nowTS := now.Format(time.RFC3339)
	record := FailedRecord{
		TS:             nowTS,
		Cycle:          fl.Cycle,
		Verdict:        VerdictFAIL,
		Classification: "cycle-mid-execution-fail",
		RecordedAt:     nowTS,
		Summary:        summary,
		Defects:        []string{summary},
		Retrospected:   true,
	}
	// ADR-0039 §7: a phase healthy enough to self-report owns its failure
	// description — its structured block beats the supervisor's synthesis.
	// Read ONCE here and thread to the deterministic-learning fallback, so
	// state.json and the lesson artifacts can never diverge on the same
	// failure event.
	structured = adoptStructuredFailure(fl.CycleState.WorkspacePath, string(fl.Failed))
	if structured != nil {
		record.Classification = structured.Class
		if len(structured.Defects) > 0 {
			record.Defects = structured.Defects
		}
	}
	// Stamp the TTL from the FINAL classification (state.go:87-91 / record.go
	// contract): without this the field is never populated, so the loop-start
	// failurelog.PruneExpiredCarryoverTodos pass keeps every entry forever and
	// the array grows unboundedly. Compute once and share so the todo inherits
	// the record's stamp rather than re-deriving it (single-sourced TTL logic).
	record.ExpiresAt = failurelog.ComputeExpiresAt(
		failurelog.NormalizeLegacy(record.Classification), now)
	appendCarryoverTodoDeduped(fl.State, CarryoverTodo{
		ID: todoID, Action: summary, Priority: carryoverPriorityBlocking,
		FirstSeenCycle: fl.Cycle, ExpiresAt: record.ExpiresAt,
	})
	fl.State.FailedAt = append(fl.State.FailedAt, record)
	fl.State.LastCycleNumber = fl.Cycle
	return summary, todoID, structured
}

func (o *Orchestrator) recordFailureLearning(ctx context.Context, fl failureLearningRequest) {
	if fl.Failed == PhaseRetro || fl.Err == nil || fl.State == nil || fl.CycleState == nil || fl.Result == nil || fl.Timings == nil {
		return
	}
	// ADR-0072 ship-phase explained-failure carrier (pipeline-defect-pipeline-
	// blocker Task 1): a ship dispatch error is a real, diagnosed failure — the
	// ONLY chokepoint a ship error ever passes through (unlike audit, ship has
	// no success-path FAIL verdict; every ship failure is an err!=nil dispatch
	// error, so this is the sole record site, mirroring persistFloorFailReasons'
	// audit-phase chokepoint). Set in orchestrator memory (never a workspace
	// file — same trust boundary as AuditFailReasons) so the coherence floor
	// can tell "audit+ACS green but ship legitimately rejected" apart from a
	// forged verdict. Cleared on ship re-dispatch by resetFloorFailReason.
	if fl.Failed == PhaseShip {
		fl.CycleState.ShipFailReasons = []string{fl.Err.Error()}
	}
	summary, todoID, structured := o.recordFailedApproachState(fl)

	// Quota deferral short-circuit (cycle-1585, instinct inst-L1582a; restores
	// the "no retro dispatched" half of the cycle-656 D2 checkpoint-and-defer
	// contract). An all-families-quota-exhausted abort
	// (cyclerun_dispatch.go:264-287) is DEFERRED, not FAILED: the quota-boundary
	// checkpoint is already written and the loop exits rc=5 so
	// `evolve loop --resume` re-enters the exhausted phase after the quota
	// window resets. Dispatching retro there would run a whole LLM phase against
	// the very wall that just drained every family, and — worse — would persist
	// CycleState.Phase/ActiveAgent as "retro", so the resume would re-enter retro
	// instead of the drained phase. Placed AFTER recordFailedApproachState so the
	// deterministic state.FailedAt / carryover-todo bookkeeping the failure
	// adapter reads is still recorded, and matched with errors.Is because the
	// sentinel arrives multiply %w-wrapped (dispatch, then wrapCycleLevelError).
	// This is the single chokepoint every such call site funnels through.
	if errors.Is(fl.Err, ErrAllFamiliesExhausted) {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: all CLI families quota-exhausted; "+
			"skipping retro dispatch (DEFERRED, resumable) — carryover todo queued only\n")
		o.writeFailureLearningState(ctx, fl.State)
		return
	}

	retroRunner, ok := o.runners[PhaseRetro]
	if !ok {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: no retro runner registered; queued carryover todo only\n")
		o.writeFailureLearningState(ctx, fl.State)
		return
	}

	retroReq := fl.retroRequest(summary, todoID)
	// S1 failure-digest assembler (ADR-0074 I2 wiring — was landed callerless by
	// cycle-1034; the digest must exist BEFORE the retro agent runs: it is the
	// identity the S2 disposition gate cross-checks and an input the agent reads).
	// Ledger load is fail-soft (nil counter → recurrence 0); a digest write
	// failure only WARNs — retro learning is never blocked by forensics plumbing.
	o.ensureFailureDigest(fl.Cycle, retroReq.ProjectRoot, fl.CycleState.WorkspacePath, string(fl.Failed), fl.Err.Error())
	retroStarted := o.now().UTC()
	fl.CycleState.Phase = string(PhaseRetro)
	fl.CycleState.PhaseStartedAt = retroStarted.Format(time.RFC3339)
	fl.CycleState.ActiveAgent = string(PhaseRetro)
	if err := o.storage.WriteCycleState(ctx, *fl.CycleState); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: write cycle-state pre-retro: %v\n", err)
	}

	cancel := o.observer.Start(ctx, string(PhaseRetro), retroReq)
	retroResp, retroErr := retroRunner.Run(ctx, retroReq)
	if cancel != nil {
		cancel()
	}
	if retroErr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: retro failed after %s failure: %v\n", fl.Failed, retroErr)
		o.writeDeterministicLearning(fl, summary, structured)
		o.writeFailureLearningState(ctx, fl.State)
		return
	}
	if !IsVerdict(retroResp.Verdict) {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: retro returned non-canonical verdict %q after %s failure\n", retroResp.Verdict, fl.Failed)
		o.writeDeterministicLearning(fl, summary, structured)
		o.writeFailureLearningState(ctx, fl.State)
		return
	}

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
	// Disposition gate (S2): a PASS retro must still deliver a valid
	// disposition.json whose failure identity agrees with the S1 digest —
	// otherwise the completion surfaces a loud gate reason instead of silently
	// recording a clean outcome (retro cannot invent or omit the disposition).
	if gateErr := o.finalizeRetroCompletion(fl.CycleState.WorkspacePath); gateErr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: %v\n", gateErr)
		fl.Result.RetroDecision = "failure-learning: " + gateErr.Error()
	} else {
		fl.Result.RetroDecision = "failure-learning: queued " + todoID
	}
	o.recordPhaseOutcome(fl.Result, fl.Timings, fl.CycleState.WorkspacePath, phaseOutcomeFrom(PhaseRetro, retroResp, 1, "", fl.CycleState.PhaseStartedAt))
	o.writeFailureLearningState(ctx, fl.State)
}

// writeDeterministicLearning is the failure floor (inbox
// retro-always-invariant, gap 1 / cycle-243): when the LLM retro cannot
// run or returns a non-canonical verdict, render the learning artifacts
// deterministically — retrospective-report.md in the cycle workspace +
// failure-lesson YAML — so the lesson survives instead of degrading to
// a stderr WARN. Best-effort: a floor write failure must never mask the
// original phase failure.
func (o *Orchestrator) writeDeterministicLearning(fl failureLearningRequest, summary string, structured *phasecontract.FailureBlock) {
	ev := faillearn.FailureEvent{
		Cycle:          fl.Cycle,
		FailedPhase:    string(fl.Failed),
		Scope:          faillearn.ScopePhase,
		Classification: "cycle-mid-execution-fail",
		Verdict:        VerdictFAIL,
		Summary:        summary,
		Defects:        []string{summary},
		EvidencePaths:  []string{fl.CycleState.WorkspacePath},
		Now:            o.now().UTC(),
	}
	// ADR-0039 §7: prefer the failed phase's own structured failure block
	// (validated + capped by adoptStructuredFailure, read ONCE by the
	// caller so state.json and the lesson cannot diverge) over the
	// synthesized summary.
	if structured != nil {
		ev.Classification = structured.Class
		if len(structured.Defects) > 0 {
			ev.Defects = structured.Defects
		}
		if len(structured.EvidencePaths) > 0 {
			ev.EvidencePaths = append(structured.EvidencePaths, fl.CycleState.WorkspacePath)
		}
	}
	lessonsDir := filepath.Join(fl.CycleRequest.ProjectRoot, ".evolve", "instincts", "lessons")
	// F1(ii): the retrospective's remediation must reach the QUEUE, not just the
	// report. Only SELF-REPORTED structured defects are filed — the synthesized
	// summary echo (ev.Defects == []string{summary}) is a restatement of the
	// failure, not an actionable item, and filing it would be inbox noise.
	//
	// The filter is faillearn.StructuredDefects, the one rule the lesson writer
	// already applies — NOT a `structured != nil` proxy for it. That proxy did
	// not implement the claim above: phasecontract.ReadFailureBlock returns a
	// block whenever Class != "", and ev.Defects is overwritten only when the
	// block carries defects, so a classed-but-defectless block left the summary
	// echo in place and filed it as a priority-H bug.
	// The near-duplicate bound is operator config, not a Go literal: resolved
	// here (the composition point that already holds projectRoot) and passed as
	// an Option so faillearn stays a policy-free leaf. A load failure resolves
	// to the compiled default, never to a disarmed or suppress-everything gate.
	pol, polErr := policy.Load(filepath.Join(fl.CycleRequest.ProjectRoot, ".evolve", "policy.json"))
	if polErr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: policy load for novelty threshold: %v (using compiled default)\n", polErr)
	}
	opts := []faillearn.Option{faillearn.WithNoveltyThreshold(pol.ResearchConfig().NoveltyThreshold)}
	if defects := faillearn.StructuredDefects(ev); len(defects) > 0 {
		if items := retroRemediationItems(fl.CycleRequest.ProjectRoot, fl.Cycle, defects); len(items) > 0 {
			opts = append(opts, faillearn.WithInbox(filepath.Join(fl.CycleRequest.ProjectRoot, ".evolve", "inbox"), items))
		}
	}
	if err := faillearn.WriteArtifacts(ev, fl.CycleState.WorkspacePath, lessonsDir, opts...); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: deterministic fallback write: %v\n", err)
	}
	o.recordRecurrenceClosure(fl.CycleRequest.ProjectRoot, ev.Classification, fl.Cycle)
}

// retroRemediationItems turns a failed phase's self-reported defects into
// inbox remediation todos (batch-integrity-review-2026-08-04.md F1(ii)): the
// 1255 defect was a retrospective that "filed" two items which never reached
// the queue, so nothing downstream could ever work them.
//
// Ids are stable per (cycle, defect) so a re-run of the floor is idempotent
// (faillearn's writeIfAbsent keeps the first write). Weight comes from policy,
// never a literal here (feedback_phase_settings_from_config_not_code); a load
// failure still files at the compiled safe default rather than dropping the
// remediation.
func retroRemediationItems(projectRoot string, cycle int, defects []string) []faillearn.InboxItem {
	pol, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: policy load for remediation weight: %v (using compiled default)\n", err)
	}
	weight := pol.RetroAutofileDefaultWeight()
	items := make([]faillearn.InboxItem, 0, len(defects))
	for _, d := range defects {
		// cycle-1282 DEF-6: defects[] and each line are agent-authored and
		// previously unbounded, so one verdict sentinel could file hundreds of
		// inbox files with megabyte titles — a queue nobody can triage is a queue
		// that hides the real item. Both caps are RECORDED below, never silent.
		if len(items) >= remediationMaxItems {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: cycle-%d self-reported %d defects; filing the first %d as remediation items and dropping the rest — fix the emitter or raise remediationMaxItems\n", cycle, len(defects), remediationMaxItems)
			break
		}
		title := truncateRunes(strings.TrimSpace(d), remediationTitleMaxRunes)
		slug := remediationSlug(title)
		if title == "" || slug == "" {
			continue // an unnameable defect yields no addressable item
		}
		items = append(items, faillearn.InboxItem{
			ID:       fmt.Sprintf("retro-%d-%s-%s", cycle, slug, remediationFingerprint(title)),
			Title:    title,
			Weight:   weight,
			Kind:     "bug",
			Priority: "H",
			// Non-empty provenance is load-bearing: inboxbatch.ConsoleRouted
			// treats an empty injected_by as operator-authored.
			InjectedBy: "faillearn-failure-floor",
		})
	}
	return items
}

// remediationSlugMaxRunes bounds the id's derived tail so a long defect line
// cannot produce an unwieldy filename. remediationMaxItems and
// remediationTitleMaxRunes bound the queue itself (cycle-1282 DEF-6).
const (
	remediationSlugMaxRunes  = 60
	remediationMaxItems      = 32
	remediationTitleMaxRunes = 500
)

// remediationFingerprint is the id's injective tail: a short digest of the
// FULL defect title, appended unconditionally.
//
// cycle-1285 F1 (HIGH). remediationSlug is lossy twice over — it stops at
// remediationSlugMaxRunes, and it collapses every run of non-alphanumerics to
// one hyphen. Two ordinary defect lines from the same subsystem that diverge
// only after rune 60 (or only in punctuation) therefore minted ONE inbox id.
// The DEF-4 collision check then correctly refused to drop the second item,
// WriteArtifacts returned before the retrospective was written, and the caller
// downgraded that to a stderr WARN: a failing cycle produced no retrospective,
// no lesson, and one item for two defects — the cycle-1255 state, reached
// through the mechanism built to prevent it, on defect text the failing agent
// chose.
//
// Unconditional rather than "only when truncated": a length test does not cover
// the punctuation collapse, and a rule with no branch cannot be wrong about
// which branch applies. The slug stays in the id because a human triaging
// `.evolve/inbox` reads it; the digest is what makes the id addressable.
func remediationFingerprint(title string) string {
	sum := sha256.Sum256([]byte(title))
	return hex.EncodeToString(sum[:])[:8]
}

// remediationSlug lowercases a defect line and maps runs of non-alphanumerics
// to single hyphens, matching the inbox's existing id shape. It is NOT
// injective — see remediationFingerprint, which is what makes the id unique.
func remediationSlug(s string) string {
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(s) {
		if b.Len() >= remediationSlugMaxRunes {
			break
		}
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevHyphen = false
		default:
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// recordRecurrenceClosure is gap-G1 production wiring (cycle-662): the
// deterministic retro-closeout seam upserts the failing lesson pattern into the
// recurrence ledger keyed by the failing cycle, so Count() finally reflects real
// history instead of staying 0 forever. Escalator/Autofiler are nil here —
// escalation APPLY stays boundary-only (the live consult site
// escalateRetroReasonForHistory reads the ledger; it must not race
// inboxmover.Claim from the mid-cycle closeout). Best-effort: a ledger failure
// must never mask the original phase failure.
func (o *Orchestrator) recordRecurrenceClosure(projectRoot, pattern string, cycle int) {
	if projectRoot == "" || strings.TrimSpace(pattern) == "" {
		return
	}
	path := filepath.Join(projectRoot, ".evolve", "recurrence-ledger.json")
	led, err := recurrence.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN recurrence: load ledger: %v\n", err)
		return
	}
	if err := led.RecordClosure(pattern, cycle, nil, nil, recurrence.DefaultEscalationPolicy()); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN recurrence: record closure: %v\n", err)
		return
	}
	if err := led.Save(path); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN recurrence: save ledger: %v\n", err)
	}
}

// adoptStructuredFailure is the trust boundary for agent-written failure
// blocks (ADR-0039 §7): adopt the failed phase's self-report ONLY when its
// class normalizes into the canonical taxonomy (never blind trust — an
// out-of-taxonomy class would round-trip to UnknownClassification on the
// next state read), and cap list/entry sizes so a misbehaving agent cannot
// bloat state.json or the lesson corpus.
func adoptStructuredFailure(workspace, phase string) *phasecontract.FailureBlock {
	fb, ok := phasecontract.ReadFailureBlock(workspace, phase)
	if !ok {
		return nil
	}
	if failurelog.NormalizeLegacy(fb.Class) == failurelog.UnknownClassification {
		return nil
	}
	fb.Defects = capStrings(fb.Defects, maxAdoptedDefects, maxAdoptedDefectRunes)
	fb.EvidencePaths = capStrings(fb.EvidencePaths, maxAdoptedDefects, maxAdoptedDefectRunes)
	return fb
}

const (
	maxAdoptedDefects     = 20  // entries per adopted list
	maxAdoptedDefectRunes = 500 // runes per adopted entry (mirrors faillearn's summary cap)
)

// capRunes truncates s to at most maxRunes runes, appending an ellipsis marker
// when truncation occurred. Single source for the rune-cap applied at every
// state.json write boundary that renders into a router/advisor prompt (adopted
// defect lists via capStrings, promoted-defect todos, carryover-todo render).
func capRunes(s string, maxRunes int) string {
	if r := []rune(s); len(r) > maxRunes {
		return string(r[:maxRunes]) + "…"
	}
	return s
}

// capStrings bounds an agent-written string list at the adoption boundary.
func capStrings(in []string, maxEntries, maxRunes int) []string {
	if len(in) > maxEntries {
		in = in[:maxEntries]
	}
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = capRunes(s, maxRunes)
	}
	return out
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
	return PhaseRequest{
		Cycle:       fl.Cycle,
		ProjectRoot: fl.CycleRequest.ProjectRoot,
		Workspace:   fl.CycleState.WorkspacePath,
		// CB.1: even this out-of-band retro keeps the no-main-tree-cwd
		// invariant — read-only, but invariants with exceptions aren't structural.
		Worktree: fl.CycleState.ActiveWorktree,
		// CB.5: and the run identity, for run-scoped session naming.
		RunID:         fl.CycleState.RunID,
		GoalHash:      fl.CycleRequest.GoalHash,
		PreviousPhase: string(fl.Failed),
		Env:           fl.Env,
		Context:       retroCtx,
	}
}

func (o *Orchestrator) writeFailureLearningState(ctx context.Context, state *State) {
	if state == nil {
		return
	}
	su, ok := o.storage.(StateUpdater)
	if !ok {
		// Legacy single-mode storage: no serialized RMW available.
		if err := o.storage.WriteState(ctx, *state); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: write state: %v\n", err)
		}
		return
	}
	// Under EVOLVE_FLEET the global cycle lock is skipped, so a peer run can write
	// state.json concurrently. Merge this run's outcome records into the on-disk
	// state (union, incoming wins per key) rather than clobbering the peer's via a
	// whole-state WriteState (which would also drop unmodeled state.json keys).
	if _, err := su.UpdateState(ctx, func(s *State) {
		s.FailedAt = mergeFailedRecords(s.FailedAt, state.FailedAt)
		s.CarryoverTodos = mergeCarryoverTodos(s.CarryoverTodos, state.CarryoverTodos)
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: update state: %v\n", err)
	}
}

// mergeFailedRecords unions disk and incoming failure records, keyed by
// (cycle, ts, verdict, recordedAt). A peer's disk-only record is preserved; the
// incoming record wins for a shared key so this run's own update (e.g. the
// Retrospected flag) is not lost. Order = disk-first appearance, then new keys.
//
// Two CONCURRENT fleet runs never collide on this key: each run holds a UNIQUE
// lease-allocated cycle number (AllocateCycleNumber — no two allocators get the
// same number), so a peer's records carry a different Cycle and key separately. A
// shared key therefore only ever identifies the SAME record (this run's updated
// copy of one it already loaded), where incoming-wins is exactly right.
func mergeFailedRecords(disk, incoming []FailedRecord) []FailedRecord {
	key := func(r FailedRecord) string {
		return fmt.Sprintf("%d\x00%s\x00%s\x00%s", r.Cycle, r.TS, r.Verdict, r.RecordedAt)
	}
	byKey := make(map[string]FailedRecord, len(disk)+len(incoming))
	order := make([]string, 0, len(disk)+len(incoming))
	add := func(r FailedRecord) {
		k := key(r)
		if _, seen := byKey[k]; !seen {
			order = append(order, k)
		}
		byKey[k] = r
	}
	for _, r := range disk {
		add(r)
	}
	for _, r := range incoming {
		add(r) // incoming overrides disk for a shared key
	}
	out := make([]FailedRecord, 0, len(order))
	for _, k := range order {
		out = append(out, byKey[k])
	}
	return out
}

// mergeCarryoverTodos unions disk and incoming todos, deduped by ID (disk-first),
// so a concurrent peer's queued todo survives this run's write.
func mergeCarryoverTodos(disk, incoming []CarryoverTodo) []CarryoverTodo {
	out := append([]CarryoverTodo(nil), disk...)
	for _, td := range incoming {
		if !carryoverTodoExists(out, td.ID) {
			out = append(out, td)
		}
	}
	return out
}

func carryoverTodoExists(todos []CarryoverTodo, id string) bool {
	for _, t := range todos {
		if t.ID == id {
			return true
		}
	}
	return false
}

// carryoverCycleTokenRE matches the cycle-number tokens the two mint sites bake
// into Action text ("cycle 1421", "cycle-1421"), so the SAME failure class
// re-minted on a later cycle fingerprints identically.
var carryoverCycleTokenRE = regexp.MustCompile(`(?i)\bcycle[ -]\d+`)

// carryoverActionFingerprint is the cross-cycle identity of a carryover todo:
// the Action text with cycle tokens normalized and whitespace/case folded.
// The 2026-08-10 investigation found 124 of 254 live entries were the same few
// failure classes duplicated per cycle (ID-keyed dedupe only), saturating the
// router prompt's 20-slot carryover window with bookkeeping noise.
func carryoverActionFingerprint(action string) string {
	s := carryoverCycleTokenRE.ReplaceAllString(action, "cycle-N")
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// carryoverFingerprintIndex returns the index of the entry sharing action's
// cross-cycle fingerprint, or -1. Cross-family suppression (a defect whose
// normalized text equals a memo/prescription carryover's) is accepted — one
// router-window slot per failure class regardless of which writer minted it.
func carryoverFingerprintIndex(todos []CarryoverTodo, action string) int {
	fp := carryoverActionFingerprint(action)
	for i, t := range todos {
		if carryoverActionFingerprint(t.Action) == fp {
			return i
		}
	}
	return -1
}

// carryoverFingerprintExists reports whether an entry with the same
// cross-cycle Action fingerprint is already tracked.
func carryoverFingerprintExists(todos []CarryoverTodo, action string) bool {
	return carryoverFingerprintIndex(todos, action) >= 0
}

// RetireCarryoverTodos is the PASS-closeout half of the carryover lifecycle:
// mergeCarryoverTodos only ever UNIONS, so an entry whose work actually shipped
// persisted forever and the router prompt's 20-slot window filled with done
// work (124 of 254 live entries were stale on 2026-08-10).
//
// An entry retires when its ID is in committedIDs, OR when it shares a retired
// entry's cross-cycle Action fingerprint — the per-cycle re-mints of the SAME
// class that the ID-keyed dedupe never collapsed. Everything else survives in
// order; the input slice is never mutated (callers re-read state under a lock,
// and a mutated input would corrupt a concurrent peer's merge).
func RetireCarryoverTodos(todos []CarryoverTodo, committedIDs []string) []CarryoverTodo {
	committed := make(map[string]bool, len(committedIDs))
	for _, id := range committedIDs {
		// A blank committed id is malformed input, not a claim about the
		// equally-malformed blank-ID entry — never let one retire the other.
		if id = strings.TrimSpace(id); id != "" {
			committed[id] = true
		}
	}
	if len(committed) == 0 || len(todos) == 0 {
		return append([]CarryoverTodo(nil), todos...)
	}

	// Pass 1: the fingerprints of the directly-committed entries. An empty
	// Action has no class identity, so it never seeds a fingerprint match.
	retiredFP := map[string]bool{}
	for _, t := range todos {
		if committed[t.ID] {
			if fp := carryoverActionFingerprint(t.Action); fp != "" {
				retiredFP[fp] = true
			}
		}
	}

	// Pass 2: drop committed ids and their same-class variants, order intact.
	out := make([]CarryoverTodo, 0, len(todos))
	for _, t := range todos {
		if committed[t.ID] || retiredFP[carryoverActionFingerprint(t.Action)] {
			continue
		}
		out = append(out, t)
	}
	return out
}

// refreshCarryoverExpiry keeps a suppressed re-mint's freshness: a class that
// keeps failing must not ride its FIRST occurrence's TTL into the boot prune
// while its duplicates are being deduped away (diff-review MEDIUM). The later
// stamp wins; an empty new stamp changes nothing.
func refreshCarryoverExpiry(t *CarryoverTodo, expiresAt string) {
	if expiresAt > t.ExpiresAt {
		t.ExpiresAt = expiresAt
	}
}

const maxFailureLearningSummaryChars = 500

// carryoverPriorityBlocking is the priority for a todo recording a failure that
// BLOCKED a cycle (a floor phase, or an errored dispatch). It outranks everything
// else in the next planner pass, which is correct for blocking work and wrong for
// advice — see carryoverPriorityLesson.
const carryoverPriorityBlocking = "P0"

// appendCarryoverTodoDeduped appends one carryover todo to state, or refreshes an
// existing one's TTL. Single-sourced (never_duplicate) between the failure path
// (recordFailedApproachState) and the judgment-lesson path (recordJudgmentLesson):
// both dedupe the same two ways — by content fingerprint first (the same failure
// re-summarized), then by todo id (the same phase failing twice in one cycle) —
// and a second copy of that logic would drift the two apart.
//
// The router prompt's own "## Carryover todos" section header already says these
// are prior-cycle items to consider before retrying, so the summary (which carries
// cycle/phase/error-class) stands alone — no boilerplate prefix repeated per todo.
func appendCarryoverTodoDeduped(state *State, todo CarryoverTodo) {
	if state == nil {
		return
	}
	if idx := carryoverFingerprintIndex(state.CarryoverTodos, todo.Action); idx >= 0 {
		refreshCarryoverExpiry(&state.CarryoverTodos[idx], todo.ExpiresAt)
		return
	}
	if carryoverTodoExists(state.CarryoverTodos, todo.ID) {
		return
	}
	state.CarryoverTodos = append(state.CarryoverTodos, todo)
}

func failureLearningSummary(cycle int, failed Phase, err error) string {
	msg := err.Error()
	r := []rune(msg)
	if len(r) > maxFailureLearningSummaryChars {
		msg = string(r[:maxFailureLearningSummaryChars]) + " ...[truncated]"
	}
	return fmt.Sprintf("cycle %d failed during %s: %s", cycle, failed, msg)
}

// ApplyDefectsAsCarryoverTodos promotes each entry in record.Defects into its
// own CarryoverTodo in state. The D2 contract requires individual defects to be
// individually addressable — one generic todo per cycle is insufficient.
func ApplyDefectsAsCarryoverTodos(state *State, record FailedRecord) {
	n := 0
	for _, defect := range record.Defects {
		if strings.TrimSpace(defect) == "" {
			continue
		}
		id := fmt.Sprintf("cycle-%d-defect-%d", record.Cycle, n)
		n++
		action := "Fix defect from cycle " + strconv.Itoa(record.Cycle) + ": " + capRunes(defect, maxAdoptedDefectRunes)
		if idx := carryoverFingerprintIndex(state.CarryoverTodos, action); idx >= 0 {
			refreshCarryoverExpiry(&state.CarryoverTodos[idx], record.ExpiresAt)
		} else if !carryoverTodoExists(state.CarryoverTodos, id) {
			state.CarryoverTodos = append(state.CarryoverTodos, CarryoverTodo{
				ID: id,
				// Bound the defect text with the SAME cap failureLearningSummary /
				// adoptStructuredFailure already apply, so an unbounded audit-gate
				// diagnostic (e.g. a long strings.Join(offenders, "; ")) can't inject
				// an arbitrarily large Action that bloats every future router prompt.
				Action:         action,
				Priority:       "P0",
				FirstSeenCycle: record.Cycle,
				CyclesUnpicked: 0,
				// Inherit the record's TTL stamp (never recompute) so the two
				// arrays' TTL logic stays single-sourced. A record with no
				// ExpiresAt leaves the todo unstamped ⇒ the prune keeps it.
				ExpiresAt: record.ExpiresAt,
			})
		}
	}
}

// fleetMode reports whether this cycle runs under the `evolve fleet` supervisor
// (EVOLVE_FLEET=1). In fleet mode the whole-cycle global project lock is not
// taken (ADR-0049 S6 / root-cause R1) so concurrent fleet cycles don't refuse
// each other; per-resource flocks + per-run isolation keep them safe. Default
// off — the single-driver loop keeps the coarse lock.
