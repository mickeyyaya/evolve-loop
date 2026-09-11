package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
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
	// StatePath is the cycle-state file this checkpoint was read from. For a
	// host-global resume it is the singleton; for a checkpoint DISCOVERED in a
	// fleet lane's per-run file it is that per-run path — and the caller must
	// route the resumed run's own state writes back to it (via
	// ipcenv.CycleStateFileKey), or the next pause orphans a checkpoint in the
	// singleton all over again.
	StatePath string
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
	// Log receives operator-facing breadcrumbs from per-run discovery (e.g.
	// "skipping stale checkpoint X, resuming older Y"). nil discards.
	Log io.Writer
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

	statePath := ResolveCycleStatePath(evolveDir) // fleet per-run override when set
	rp, err := loadResumeStateFrom(statePath, projectRoot, opts)
	if err == nil {
		return rp, nil
	}
	// Discovery fallback — the 2026-08-29 incident. Fleet lanes write their
	// quota/escalation checkpoints through the SAME resolver, but with
	// ipcenv.CycleStateFileKey set, so the checkpoint lands in the lane's
	// per-run cycle-state file. A later host-global `evolve loop --resume`
	// (fresh process, no override) resolved only the singleton and reported
	// "no live checkpoint" while three live quota-likely checkpoints sat in
	// .evolve/runs/cycle-158*/cycle-state.json — abandoning a lane that had
	// already completed build and reached audit. The writer learned fleet
	// isolation; the reader had not.
	//
	// Scope, deliberately narrow:
	//   - only on ErrNoCheckpoint (a stale PRIMARY checkpoint is a real answer
	//     about a real checkpoint — never scan past it);
	//   - only when NO env override is set: inside a lane the override IS the
	//     authority, and scanning siblings would let one lane resume another's
	//     cycle.
	if !errors.Is(err, ErrNoCheckpoint) || os.Getenv(ipcenv.CycleStateFileKey) != "" {
		return nil, err
	}
	rp, derr := discoverPerRunResumeState(evolveDir, projectRoot, opts)
	if derr != nil {
		if errors.Is(derr, errNoPerRunCandidates) {
			// Nothing anywhere — keep the primary error, extended with where
			// discovery looked, so the operator does not rediscover the split.
			return nil, fmt.Errorf("%w (also scanned %s for fleet per-run checkpoints: none live)", err, filepath.Join(evolveDir, "runs"))
		}
		return nil, derr
	}
	return rp, nil
}

// errNoPerRunCandidates reports that the per-run scan found no live
// checkpoint blocks at all (as opposed to finding one that failed validation).
var errNoPerRunCandidates = errors.New("resume: no per-run checkpoint candidates")

// resumableReasons are the checkpoint reasons another process may legitimately
// resume: the ESCALATION pauses, written once when a run deliberately stops.
//
// "phase-complete" is deliberately absent. PhaseBoundaryCheckpointer writes an
// enabled phase-complete block after EVERY phase of a HEALTHY run — it is a
// crash-recovery breadcrumb for the SAME process, and its shape defeats the
// stale-checks (gitHead is always "", and a live lane's worktree exists), so
// admitting it would let a host-global --resume double-drive a running lane,
// or silently resurrect a cycle that already ran to a terminal FAIL.
var resumableReasons = map[string]bool{
	"quota-likely":       true,
	"batch-cap-near":     true,
	"operator-requested": true,
	"stall-inactivity":   true,
}

// discoverPerRunResumeState scans evolveDir/runs/*/cycle-state.json for
// checkpoints another process may resume: enabled, an escalation reason
// (resumableReasons), and NO fresh lease — the run-dir heartbeat gc already
// trusts (runlease.Fresh); a fresh lease means the lane is alive right now and
// resuming it would double-drive the cycle. A quota-paused lane's process has
// exited, so its heartbeat is stale and it stays discoverable.
//
// Candidates load newest-first (savedAt, then cycle_id) and the newest VALID
// one wins. Lanes are independent — no supersession — so when the newest is
// stale an older valid sibling is resumed, with a breadcrumb on opts.Log
// naming the skipped one (the operator must learn it needs attention NOW, not
// when a later resume trips over it). Only when every candidate fails does the
// newest's error return — stale must say stale, because "nothing to resume"
// tells the operator to relaunch fresh and burn the preserved progress.
func discoverPerRunResumeState(evolveDir, projectRoot string, opts ResumeOptions) (*ResumePoint, error) {
	entries, rerr := os.ReadDir(filepath.Join(evolveDir, "runs"))
	if rerr != nil {
		return nil, errNoPerRunCandidates
	}
	type candidate struct {
		path    string
		savedAt string
		cycle   int
	}
	var cands []candidate
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(evolveDir, "runs", e.Name(), CycleStateFile)
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var blob map[string]any
		if json.Unmarshal(raw, &blob) != nil {
			continue
		}
		cp, ok := blob["checkpoint"].(map[string]any)
		if !ok {
			continue
		}
		if enabled, _ := cp["enabled"].(bool); !enabled {
			continue
		}
		if !resumableReasons[strFromAny(cp["reason"])] {
			continue
		}
		if l, ok, _ := runlease.Read(filepath.Join(evolveDir, "runs", e.Name())); ok && runlease.Fresh(l, time.Now(), 0) {
			continue // lane is ALIVE — never resume out from under it
		}
		cands = append(cands, candidate{path: path, savedAt: strFromAny(cp["savedAt"]), cycle: intFromAny(blob["cycle_id"])})
	}
	if len(cands) == 0 {
		return nil, errNoPerRunCandidates
	}
	// Newest first. savedAt is RFC3339, so string order IS time order; the
	// cycle id breaks ties (two lanes checkpointed in the same second).
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].savedAt != cands[j].savedAt {
			return cands[i].savedAt > cands[j].savedAt
		}
		return cands[i].cycle > cands[j].cycle
	})
	var firstErr error
	log := opts.Log
	if log == nil {
		log = io.Discard
	}
	for _, c := range cands {
		rp, err := loadResumeStateFrom(c.path, projectRoot, opts)
		if err == nil {
			return rp, nil
		}
		fmt.Fprintf(log, "[resume] skipping %s: %v\n", c.path, err)
		if firstErr == nil {
			firstErr = fmt.Errorf("per-run checkpoint %s: %w", c.path, err)
		}
	}
	return nil, firstErr
}

// loadResumeStateFrom reads ONE state file, extracts the checkpoint block,
// validates HEAD + worktree, and returns a ResumePoint. This is the former
// body of LoadResumeState, extracted verbatim so the primary path and the
// per-run discovery share one loader and cannot drift.
func loadResumeStateFrom(statePath, projectRoot string, opts ResumeOptions) (*ResumePoint, error) {
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

	if strFromAny(blob["phase"]) == string(PhaseEnd) {
		return nil, fmt.Errorf("%w: cycle already completed", ErrNoCheckpoint)
	}
	cp, ok := blob["checkpoint"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: cycle-state.json has no checkpoint block", ErrNoCheckpoint)
	}
	if enabled, _ := cp["enabled"].(bool); !enabled {
		return nil, fmt.Errorf("%w: checkpoint.enabled != true", ErrNoCheckpoint)
	}

	rp := &ResumePoint{
		StatePath:       statePath,
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

// ActivateResumeStatePath points this process's cycle-state resolution at the
// file rp's checkpoint was read from, when that differs from the current
// resolution — the write-back half of per-run checkpoint discovery. A resumed
// cycle that keeps writing to the host-global singleton while its checkpoint
// lives in a per-run file re-creates the split this feature closes: its next
// quota pause would checkpoint one place and be sought in another.
//
// It reuses the SAME mechanism a fleet lane uses at spawn
// (ipcenv.CycleStateFileKey, see cyclerun.go) rather than a second channel.
// The returned restore func puts the previous value back; a no-op restore is
// returned when nothing needed to change.
func ActivateResumeStatePath(rp *ResumePoint, evolveDir string) func() {
	if rp == nil || rp.StatePath == "" || rp.StatePath == ResolveCycleStatePath(evolveDir) {
		return func() {}
	}
	prev, had := os.LookupEnv(ipcenv.CycleStateFileKey)
	_ = os.Setenv(ipcenv.CycleStateFileKey, rp.StatePath)
	return func() {
		if had {
			_ = os.Setenv(ipcenv.CycleStateFileKey, prev)
		} else {
			_ = os.Unsetenv(ipcenv.CycleStateFileKey)
		}
	}
}

// RunCycleFromPhase resumes an in-flight cycle starting at the given
// phase. Skips state-machine traversal of completedPhases and replays
// from `phase` onward through the rest of the cycle.
//
// Unlike RunCycle, this method never allocates a cycle number: it completes
// the original run and advances the completed cursor monotonically. It acquires
// the normal storage lock before reading state and shares terminal closeout.
func (o *Orchestrator) RunCycleFromPhase(ctx context.Context, req CycleRequest, resumePoint *ResumePoint) (result CycleResult, retErr error) {
	if err := o.ensureSafeConfig(); err != nil {
		return CycleResult{}, err
	}
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

	boot, err := o.loadResumeBootstrap(ctx, req, resumePoint, startPhase)
	if err != nil {
		return CycleResult{}, err
	}
	req = boot.request
	state := boot.state
	cs := boot.cycleState
	startPhase = boot.startPhase
	cycle := boot.cycle

	o.currentRunID.Store(cs.RunID)
	defer o.currentRunID.Store("")
	stopLease := startRunLease(cs.WorkspacePath, cs.RunID, o.now, leaseRefreshInterval())
	defer stopLease()

	inputs := o.snapshotResumeInputs(ctx, &boot)
	cs = boot.cycleState
	envSnap := inputs.env
	ctxSnap := inputs.context
	result = inputs.result
	preResumeHEAD := inputs.preResumeHEAD
	mainDirtyBaseline := inputs.mainDirtyBaseline

	execution := resumeExecution{
		orchestrator:      o,
		ctx:               ctx,
		request:           req,
		resumePoint:       resumePoint,
		state:             state,
		cycleState:        cs,
		cycle:             cycle,
		startPhase:        startPhase,
		envSnapshot:       envSnap,
		contextSnapshot:   ctxSnap,
		initialResult:     result,
		preResumeHEAD:     preResumeHEAD,
		mainDirtyBaseline: mainDirtyBaseline,
	}
	return execution.run()

}

func (o *Orchestrator) reviewResumedDeliverable(
	ctx context.Context,
	projectRoot string,
	cycle int,
	cs CycleState,
	phase Phase,
	runner PhaseRunner,
	req PhaseRequest,
	resp PhaseResponse,
	mainDirtyBaseline map[string]bool,
) (PhaseResponse, error) {
	// Resume review parity is scoped to activated contracts. Legacy checkpoints
	// retain their historical behavior.
	if o.reviewer == nil || cs.ExplanationDocumentationVersion == 0 || resp.Verdict == VerdictSKIPPED {
		return resp, nil
	}
	recoverBeforeReview := func() error {
		if !o.leakRecoverablePhase(phase) || cs.ActiveWorktree == "" {
			return nil
		}
		if recoverBuildLeak(ctx, projectRoot, cs.ActiveWorktree, mainDirtyBaseline, o.worktreePhase(phase)) {
			return nil
		}
		return fmt.Errorf("phase %s: worktree-leak recovery failed (main tree left unsafe for review and audit)", phase)
	}
	reviewInput := func(response PhaseResponse) ReviewInput {
		return ReviewInput{
			Cycle: cycle, RunID: cs.RunID,
			ExplanationDocumentationVersion: cs.ExplanationDocumentationVersion,
			Phase:                           string(phase), WorktreeBaseSHA: cs.WorktreeBaseSHA,
			Response: response, Workspace: cs.WorkspacePath,
			Worktree: cs.ActiveWorktree, ProjectRoot: projectRoot,
		}
	}
	if err := recoverBeforeReview(); err != nil {
		return resp, err
	}
	review := o.reviewer.Review(ctx, reviewInput(resp))
	maxCorrections := (&cycleRun{o: o, cs: cs, retryConfig: o.retryConfig}).correctionLimitFor(phase, o.retryConfig.ContractCorrectionRetries)
	for correction := 1; !review.Approve && correction <= maxCorrections; correction++ {
		req.CorrectionDirective = composeCorrection(review.Reason, review.Remediation)
		cancel := o.observer.Start(ctx, string(phase), req)
		corrected, err := runner.Run(ctx, req)
		if cancel != nil {
			cancel()
		}
		if err != nil {
			return corrected, fmt.Errorf("resume review gate: phase %q correction %d dispatch failed: %w", phase, correction, err)
		}
		if !IsVerdict(corrected.Verdict) {
			return corrected, fmt.Errorf("resume review gate: phase %q correction %d returned non-canonical verdict %q", phase, correction, corrected.Verdict)
		}
		resp = corrected
		if err := recoverBeforeReview(); err != nil {
			return resp, err
		}
		// Correction output is a fresh worktree mutation. Normalize it before
		// re-running the reviewer so a newly sealed snapshot is final.
		o.normalizeBuildWorktree(ctx, phase, cs)
		review = o.reviewer.Review(ctx, reviewInput(resp))
	}
	if !review.Approve {
		return resp, fmt.Errorf("resume review gate: phase %q deliverable rejected after %d correction(s): %s", phase, maxCorrections, review.Reason)
	}
	return resp, nil
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
