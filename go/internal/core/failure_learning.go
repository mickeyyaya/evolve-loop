package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/faillearn"
	"github.com/mickeyyaya/evolveloop/go/internal/failuregrade"
	"github.com/mickeyyaya/evolveloop/go/internal/failurelog"
	"github.com/mickeyyaya/evolveloop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolveloop/go/internal/recovery"
)

type phaseTimingEntry struct {
	Phase      string `json:"phase"`
	DurationMS int64  `json:"duration_ms"`
	// BootMS is the cold REPL-boot slice of DurationMS (ADR-0043 A0) — pure
	// dispatch overhead before the model worked. omitempty: 0 for warm/headless.
	BootMS       int64   `json:"boot_ms,omitempty"`
	Verdict      string  `json:"verdict"`
	CostUSD      float64 `json:"cost_usd"`
	AttemptCount int     `json:"attempt_count"`
	// AbortReason is set when the cycle aborted AFTER this phase produced its
	// outcome (ADR-0044 C1): the verdict above stays the agent's own; the
	// abort is a cycle-level disposition. omitempty: absent on happy paths.
	AbortReason string `json:"abort_reason,omitempty"`
}

type phaseUsageSidecar struct {
	Phase        string  `json:"phase"`
	CostUSD      float64 `json:"cost_usd"`
	DurationMS   int64   `json:"duration_ms"`
	AttemptCount int     `json:"attempt_count"`
	Verdict      string  `json:"verdict"`
	// AbortReason mirrors phaseTimingEntry.AbortReason (ADR-0044 C1).
	AbortReason string `json:"abort_reason,omitempty"`
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
func phaseOutcomeFrom(phase Phase, resp PhaseResponse, attempts int, abortReason string) recovery.PhaseOutcome {
	verdict := resp.Verdict
	if !IsVerdict(verdict) {
		verdict = VerdictFAIL
	}
	return recovery.PhaseOutcome{
		Phase:        string(phase),
		Verdict:      verdict,
		CostUSD:      resp.CostUSD,
		DurationMS:   resp.DurationMS,
		BootMS:       resp.BootMS,
		AttemptCount: attempts,
		AbortReason:  abortReason,
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
	result.PhasesRun = append(result.PhasesRun, Phase(out.Phase))
	*timings = append(*timings, phaseTimingEntry{
		Phase:        out.Phase,
		DurationMS:   out.DurationMS,
		BootMS:       out.BootMS,
		Verdict:      out.Verdict,
		CostUSD:      out.CostUSD,
		AttemptCount: out.AttemptCount,
		AbortReason:  out.AbortReason,
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
		AbortReason:  out.AbortReason,
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
	timingPath := filepath.Join(workspace, "phase-timing.json")
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
	Phase        string `json:"phase"`
	Cycle        int    `json:"cycle"`
	ErrorMessage string `json:"error_message"`
	ExitCode     int    `json:"exit_code"`
	AttemptCount int    `json:"attempt_count"`
	Timestamp    string `json:"timestamp"`
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
		Phase:        phase,
		Cycle:        cycle,
		ErrorMessage: phaseErr.Error(),
		ExitCode:     exitCode,
		AttemptCount: attempts,
		Timestamp:    now().UTC().Format(time.RFC3339),
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

func (o *Orchestrator) recordFailureLearning(ctx context.Context, fl failureLearningRequest) {
	if fl.Failed == PhaseRetro || fl.Err == nil || fl.State == nil || fl.CycleState == nil || fl.Result == nil || fl.Timings == nil {
		return
	}
	summary := failureLearningSummary(fl.Cycle, fl.Failed, fl.Err)
	todoID := fmt.Sprintf("cycle-%d-failed-%s", fl.Cycle, fl.Failed)
	if !carryoverTodoExists(fl.State.CarryoverTodos, todoID) {
		fl.State.CarryoverTodos = append(fl.State.CarryoverTodos, CarryoverTodo{
			ID:             todoID,
			Action:         "Review the failed cycle learning and fix before retrying: " + summary,
			Priority:       "P0",
			FirstSeenCycle: fl.Cycle,
			CyclesUnpicked: 0,
		})
	}
	now := o.now().UTC().Format(time.RFC3339)
	record := FailedRecord{
		TS:             now,
		Cycle:          fl.Cycle,
		Verdict:        VerdictFAIL,
		Classification: "cycle-mid-execution-fail",
		RecordedAt:     now,
		Summary:        summary,
		Defects:        []string{summary},
		Retrospected:   true,
	}
	// ADR-0039 §7: a phase healthy enough to self-report owns its failure
	// description — its structured block beats the supervisor's synthesis.
	// Read ONCE here and thread to the deterministic-learning fallback, so
	// state.json and the lesson artifacts can never diverge on the same
	// failure event.
	structured := adoptStructuredFailure(fl.CycleState.WorkspacePath, string(fl.Failed))
	if structured != nil {
		record.Classification = structured.Class
		if len(structured.Defects) > 0 {
			record.Defects = structured.Defects
		}
	}
	fl.State.FailedAt = append(fl.State.FailedAt, record)
	fl.State.LastCycleNumber = fl.Cycle

	retroRunner, ok := o.runners[PhaseRetro]
	if !ok {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: no retro runner registered; queued carryover todo only\n")
		o.writeFailureLearningState(ctx, fl.State)
		return
	}

	retroReq := fl.retroRequest(summary, todoID)
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
	fl.Result.RetroDecision = "failure-learning: queued " + todoID
	o.recordPhaseOutcome(fl.Result, fl.Timings, fl.CycleState.WorkspacePath, phaseOutcomeFrom(PhaseRetro, retroResp, 1, ""))
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
	if err := faillearn.WriteArtifacts(ev, fl.CycleState.WorkspacePath, lessonsDir); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN failure-learning: deterministic fallback write: %v\n", err)
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

// capStrings bounds an agent-written string list at the adoption boundary.
func capStrings(in []string, maxEntries, maxRunes int) []string {
	if len(in) > maxEntries {
		in = in[:maxEntries]
	}
	out := make([]string, len(in))
	for i, s := range in {
		if r := []rune(s); len(r) > maxRunes {
			s = string(r[:maxRunes]) + "…"
		}
		out[i] = s
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

const maxFailureLearningSummaryChars = 500

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
		if !carryoverTodoExists(state.CarryoverTodos, id) {
			state.CarryoverTodos = append(state.CarryoverTodos, CarryoverTodo{
				ID:             id,
				Action:         "Fix defect from cycle " + strconv.Itoa(record.Cycle) + ": " + defect,
				Priority:       "P0",
				FirstSeenCycle: record.Cycle,
				CyclesUnpicked: 0,
			})
		}
	}
}

// fleetMode reports whether this cycle runs under the `evolve fleet` supervisor
// (EVOLVE_FLEET=1). In fleet mode the whole-cycle global project lock is not
// taken (ADR-0049 S6 / root-cause R1) so concurrent fleet cycles don't refuse
// each other; per-resource flocks + per-run isolation keep them safe. Default
// off — the single-driver loop keeps the coarse lock.
