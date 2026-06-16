package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

// ResumePoint describes a checkpointed cycle that can be resumed.
// Field shape mirrors the relevant subset of
// .evolve/cycle-state.json:checkpoint plus the cycle_id + project_root
// the operator needs to drive the resume.
type ResumePoint struct {
	CycleID         int      // cycle_id at top of cycle-state.json
	Phase           string   // checkpoint.resumeFromPhase
	WorktreePath    string   // checkpoint.worktreePath
	GitHead         string   // checkpoint.gitHead (at pause)
	CompletedPhases []string // checkpoint.completedPhases
	CostAtPause     float64  // checkpoint.costAtCheckpoint
	Reason          string   // checkpoint.reason
	SavedAt         string   // checkpoint.savedAt
	AutoAttempts    int      // checkpoint.autoResumeAttempts (post-bump)
	AutoMaxAttempts int      // checkpoint.autoResumeMaxAttempts
}

// ResumeOptions wires test seams + operator overrides for LoadResumeState.
type ResumeOptions struct {
	// AllowHeadMoved corresponds to EVOLVE_RESUME_ALLOW_HEAD_MOVED=1.
	// When true, a current-HEAD vs checkpoint-HEAD mismatch is a WARN,
	// not a hard fail.
	AllowHeadMoved bool
	// CurrentHead returns the current git HEAD for projectRoot. Defaults
	// to `git rev-parse HEAD`. Tests inject deterministic values.
	CurrentHead func(projectRoot string) (string, error)
	// PathExists tests whether a worktree path is still on disk.
	// Defaults to os.Stat.
	PathExists func(path string) bool
}

// ErrNoCheckpoint is returned when cycle-state.json lacks a usable
// checkpoint block. Operator-facing message: "nothing to resume".
var ErrNoCheckpoint = errors.New("resume: no live checkpoint")

// ErrStaleCheckpoint is returned when validation fails: HEAD drifted
// without the override, worktree missing, or required fields absent.
var ErrStaleCheckpoint = errors.New("resume: checkpoint stale")

// LoadResumeState reads .evolve/cycle-state.json under evolveDir,
// extracts the checkpoint block, validates git HEAD + worktree, and
// returns a ResumePoint. Mirrors resume-cycle.sh:71-110.
//
// projectRoot is the writable host repo (where git lives). evolveDir is
// typically projectRoot + "/.evolve" but is passed separately so
// tests can place a synthetic state file anywhere.
func LoadResumeState(_ context.Context, projectRoot, evolveDir string, opts ResumeOptions) (*ResumePoint, error) {
	if opts.CurrentHead == nil {
		opts.CurrentHead = defaultCurrentHead
	}
	if opts.PathExists == nil {
		opts.PathExists = defaultPathExists
	}

	statePath := filepath.Join(evolveDir, CycleStateFile)
	raw, err := os.ReadFile(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s not found", ErrNoCheckpoint, statePath)
		}
		return nil, fmt.Errorf("resume: read state: %w", err)
	}

	var blob map[string]any
	if err := json.Unmarshal(raw, &blob); err != nil {
		return nil, fmt.Errorf("resume: parse state: %w", err)
	}

	cp, ok := blob["checkpoint"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: cycle-state.json has no checkpoint block", ErrNoCheckpoint)
	}
	if enabled, _ := cp["enabled"].(bool); !enabled {
		return nil, fmt.Errorf("%w: checkpoint.enabled != true", ErrNoCheckpoint)
	}

	rp := &ResumePoint{
		CycleID:         intFromAny(blob["cycle_id"]),
		Phase:           strFromAny(cp["resumeFromPhase"]),
		WorktreePath:    strFromAny(cp["worktreePath"]),
		GitHead:         strFromAny(cp["gitHead"]),
		CompletedPhases: stringsFromAny(cp["completedPhases"]),
		CostAtPause:     floatFromAny(cp["costAtCheckpoint"]),
		Reason:          strFromAny(cp["reason"]),
		SavedAt:         strFromAny(cp["savedAt"]),
		AutoAttempts:    intFromAny(cp["autoResumeAttempts"]),
		AutoMaxAttempts: intFromAny(cp["autoResumeMaxAttempts"]),
	}
	if rp.Phase == "" {
		return nil, fmt.Errorf("%w: checkpoint.resumeFromPhase missing", ErrStaleCheckpoint)
	}

	// HEAD validation. checkpoint.gitHead == "unknown" means the original
	// capture failed (rare); skip validation in that case.
	if rp.GitHead != "" && rp.GitHead != "unknown" {
		current, err := opts.CurrentHead(projectRoot)
		if err == nil && strings.TrimSpace(current) != rp.GitHead && !opts.AllowHeadMoved {
			return nil, fmt.Errorf("%w: git HEAD moved (was %s, now %s); set AllowHeadMoved to override",
				ErrStaleCheckpoint, rp.GitHead, strings.TrimSpace(current))
		}
	}

	// Worktree validation. Empty/null worktree path skips the check
	// (cycle didn't use a per-cycle worktree).
	if rp.WorktreePath != "" && !opts.PathExists(rp.WorktreePath) {
		return nil, fmt.Errorf("%w: worktree %s no longer exists",
			ErrStaleCheckpoint, rp.WorktreePath)
	}

	return rp, nil
}

// RunCycleFromPhase resumes an in-flight cycle starting at the given
// phase. Skips state-machine traversal of completedPhases and replays
// from `phase` onward through the rest of the cycle.
//
// Unlike RunCycle, RunCycleFromPhase does NOT increment LastCycleNumber
// — it operates on the cycle that's already in flight. It also does
// NOT re-acquire the cycle lock (the caller already holds it, since the
// checkpoint was written under lock).
func (o *Orchestrator) RunCycleFromPhase(ctx context.Context, req CycleRequest, resumePoint *ResumePoint) (CycleResult, error) {
	if resumePoint == nil {
		return CycleResult{}, fmt.Errorf("RunCycleFromPhase: resumePoint required")
	}
	startPhase := Phase(resumePoint.Phase)
	_, inRunners := o.runners[startPhase]
	if (!startPhase.IsValid() && !inRunners) || startPhase == PhaseEnd || startPhase == PhaseStart {
		return CycleResult{}, fmt.Errorf("RunCycleFromPhase: invalid resume phase %q", resumePoint.Phase)
	}

	// Lock + state read (consistent with RunCycle's invariants).
	release, err := o.storage.AcquireLock(ctx)
	if err != nil {
		return CycleResult{}, fmt.Errorf("acquire lock: %w", err)
	}
	defer func() { _ = release() }()

	state, err := o.storage.ReadState(ctx)
	if err != nil {
		return CycleResult{}, fmt.Errorf("read state: %w", err)
	}

	cs, err := o.storage.ReadCycleState(ctx)
	if err != nil {
		return CycleResult{}, fmt.Errorf("read cycle-state: %w", err)
	}
	cycle := cs.CycleID
	if cycle == 0 {
		cycle = resumePoint.CycleID
	}
	// CA.5: resume REUSES the run record's identity — the resumed phases'
	// ledger entries attribute to the original run. A pre-CA.5 record (no
	// run_id) mints fresh rather than leaving entries unattributed.
	if cs.RunID == "" {
		cs.RunID = MintRunID(o.now())
	}
	o.currentRunID.Store(cs.RunID)
	defer o.currentRunID.Store("")

	// ADR-0049 G16: re-establish the per-run .lease for the resumed (still-live)
	// cycle so gc does not reap its run dir while resume runs. Same heartbeat as
	// the fresh path; no-op for worktree-less cycles.
	stopLease := startRunLease(cs.WorkspacePath, cs.RunID, o.now, leaseRefreshInterval())
	defer stopLease()

	// Snapshot env/context (same discipline as RunCycle).
	envSnap := make(map[string]string, len(req.Env)+1)
	for k, v := range req.Env {
		envSnap[k] = v
	}
	envSnap["EVOLVE_RESUME_MODE"] = "1"
	ctxSnap := make(map[string]string, len(req.Context))
	for k, v := range req.Context {
		ctxSnap[k] = v
	}

	result := CycleResult{Cycle: cycle, FinalVerdict: VerdictPASS}

	// ADR-0044 C1 (deferred-to-C3 debt, now paid): the resume path was a
	// SECOND recording boundary that wrote no timings/sidecars at all —
	// resumed phases were invisible in phase-timing.json. Every terminal
	// disposition below funnels through the same recordPhaseOutcome
	// chokepoint RunCycle uses; the deferred writer flushes on abort too
	// and APPEND-MERGES with the pre-crash entries (writePhaseTimings).
	// Semantic note: PhasesRun now includes aborted-but-DISPATCHED phases on
	// resume too (the chokepoint appends on every terminal path) — same
	// what-actually-ran contract RunCycle adopted in Slice 1; consumers are
	// printing/telemetry only (audited then).
	var phaseTimings []phaseTimingEntry
	defer func() {
		if len(phaseTimings) == 0 {
			return
		}
		writePhaseTimings(cs.WorkspacePath, phaseTimings)
	}()

	// Synthesize the loop: start from `startPhase`, follow the state
	// machine forward like RunCycle does.
	current := startPhase
	lastVerdict := VerdictPASS
	var scheduledNext Phase

	// Run the start phase first, then continue with state-machine.
	first := true
	for safety := 0; safety < 32; safety++ {
		var next Phase
		switch {
		case first:
			next = current
			first = false
		case scheduledNext != "":
			next = scheduledNext
			scheduledNext = ""
		default:
			n, err := o.sm.Next(current, lastVerdict)
			if err != nil {
				return result, fmt.Errorf("transition from %s: %w", current, err)
			}
			next = n
		}
		if next == PhaseEnd {
			break
		}

		runner, ok := o.runners[next]
		if !ok {
			return result, fmt.Errorf("%w: no runner registered for phase %s", ErrPhaseInvalid, next)
		}

		cs.Phase = string(next)
		cs.ActiveAgent = string(next)
		if err := o.storage.WriteCycleState(ctx, cs); err != nil {
			return result, fmt.Errorf("write cycle-state pre-%s: %w", next, err)
		}

		resp, err := runner.Run(ctx, PhaseRequest{
			Cycle:       cycle,
			ProjectRoot: req.ProjectRoot,
			Workspace:   cs.WorkspacePath,
			// CB.1: the resume path is a first-class dispatch surface and must
			// thread the persisted worktree like the RunCycle loop does — a
			// resumed phase with Worktree="" runs cwd=main-tree (cycle-280 class).
			Worktree: cs.ActiveWorktree,
			// CB.5: same rule for the persisted run identity (resume reuses
			// the run-record id, so session names stay run-scoped).
			RunID:         cs.RunID,
			GoalHash:      req.GoalHash,
			PreviousPhase: string(current),
			Env:           envSnap,
			Context:       ctxSnap,
		})
		if err != nil {
			phaseErr := fmt.Errorf("phase %s: %w", next, err)
			o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, 1, phaseErr.Error()))
			return result, phaseErr
		}
		if !IsVerdict(resp.Verdict) {
			ferr := fmt.Errorf("phase %s returned non-canonical verdict %q", next, resp.Verdict)
			o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, 1, ferr.Error()))
			return result, ferr
		}

		if err := o.ledger.Append(ctx, LedgerEntry{
			TS:       o.now().UTC().Format(time.RFC3339),
			Cycle:    cycle,
			Role:     string(next),
			Kind:     "phase",
			ExitCode: 0,
		}); err != nil {
			lerr := fmt.Errorf("ledger append for %s: %w", next, err)
			o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, 1, lerr.Error()))
			return result, lerr
		}

		o.emitPhaseBindings(ctx, cycle, req.ProjectRoot, cs, next, resp.Verdict)
		// Cycle-156 parity: same post-build normalize RunCycle runs, driven
		// by the persisted CycleState.WorktreeBaseSHA (empty on pre-field
		// checkpoints → no-op).
		o.normalizeBuildWorktree(ctx, next, cs)

		cs.CompletedPhases = append(cs.CompletedPhases, string(next))
		if err := o.storage.WriteCycleState(ctx, cs); err != nil {
			werr := fmt.Errorf("write cycle-state post-%s: %w", next, err)
			o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, 1, werr.Error()))
			return result, werr
		}

		result.FinalVerdict = resp.Verdict
		o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, 1, ""))
		current = next
		lastVerdict = resp.Verdict

		if current == PhaseRetro {
			branch, extraEnv, reason := o.decideAfterRetro(resp.Verdict, state.FailedAt)
			for k, v := range extraEnv {
				envSnap[k] = v
			}
			result.RetroDecision = reason
			if branch == PhaseEnd {
				break
			}
			if !o.sm.CanTransition(PhaseRetro, branch) {
				return result, fmt.Errorf("retro→%s not allowed by state machine", branch)
			}
			scheduledNext = branch
		}
	}

	// Resume completed — preserve LastCycleNumber (already advanced when
	// the original cycle started; resume doesn't re-advance it).
	if err := o.storage.WriteState(ctx, state); err != nil {
		return result, fmt.Errorf("write state: %w", err)
	}
	return result, nil
}

// --- helpers ---

func defaultCurrentHead(projectRoot string) (string, error) {
	// Capture (not gitexec HEAD/Output) preserves the historical UNTRIMMED return
	// — callers receive the raw `git rev-parse HEAD` stdout (trailing newline and
	// all), as the pre-S4.5 cmd.Output() form did.
	out, stderr, code, err := gitexec.Git{Dir: projectRoot, Exec: gitRunner}.Capture(context.Background(), "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if code != 0 {
		return out, fmt.Errorf("git rev-parse HEAD: rc=%d: %s", code, strings.TrimSpace(stderr))
	}
	return out, nil
}

func defaultPathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func strFromAny(v any) string {
	s, _ := v.(string)
	return s
}

func intFromAny(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

func floatFromAny(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}

func stringsFromAny(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
