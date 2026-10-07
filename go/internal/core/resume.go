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
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// ResumePoint describes a checkpointed cycle that can be resumed.
// Field shape mirrors the relevant subset of
// .evolve/cycle-state.json:checkpoint plus the cycle_id + project_root
// the operator needs to drive the resume.
type ResumePoint struct {
	CycleID         int
	Phase           string
	WorktreePath    string
	GitHead         string
	CompletedPhases []string
	CostAtPause     float64
	Reason          string
	SavedAt         string
	AutoAttempts    int
	AutoMaxAttempts int
	StatePath       string
}

// ResumeOptions wires test seams + operator overrides for LoadResumeState.
type ResumeOptions struct {
	AllowHeadMoved bool
	CurrentHead    func(projectRoot string) (string, error)
	PathExists     func(path string) bool
	Log            io.Writer
}

// ErrNoCheckpoint is returned when cycle-state.json lacks a usable
// checkpoint block. Operator-facing message: "nothing to resume".
var ErrNoCheckpoint = errors.New("resume: no live checkpoint")

// ErrStaleCheckpoint is returned when validation fails: HEAD drifted
// without the override, worktree missing, or required fields absent.
var ErrStaleCheckpoint = errors.New("resume: checkpoint stale")

// LoadResumeState reads .evolve/cycle-state.json under evolveDir,
// extracts the checkpoint block, validates git HEAD + worktree, and
// returns a ResumePoint.
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

	statePath := ResolveCycleStatePath(evolveDir)
	rp, err := loadResumeStateFrom(statePath, projectRoot, opts)
	if err == nil {
		return rp, nil
	}
	if !errors.Is(err, ErrNoCheckpoint) || statePath != paths.CycleStateFileFor(evolveDir, "") {
		return nil, err
	}
	rp, derr := discoverPerRunResumeState(evolveDir, projectRoot, opts)
	if derr != nil {
		if errors.Is(derr, errNoPerRunCandidates) {
			return nil, fmt.Errorf("%w (also scanned %s for fleet per-run checkpoints: none live)", err, filepath.Join(evolveDir, "runs"))
		}
		return nil, derr
	}
	return rp, nil
}

// errNoPerRunCandidates reports that the per-run scan found no live
// checkpoint blocks at all (as opposed to finding one that failed validation).
var errNoPerRunCandidates = errors.New("resume: no per-run checkpoint candidates")

var resumableReasons = map[string]bool{
	"quota-likely":       true,
	"batch-cap-near":     true,
	"operator-requested": true,
	"stall-inactivity":   true,
}

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
			continue
		}
		cands = append(cands, candidate{path: path, savedAt: strFromAny(cp["savedAt"]), cycle: intFromAny(blob["cycle_id"])})
	}
	if len(cands) == 0 {
		return nil, errNoPerRunCandidates
	}
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

// loadResumeStateFrom reads one state file, validates HEAD + worktree, and
// returns a ResumePoint; the primary and per-run-discovery paths share this
// loader so they cannot drift.
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

	// checkpoint.gitHead == "unknown" means the original capture failed; skip
	// validation in that case.
	if rp.GitHead != "" && rp.GitHead != "unknown" {
		current, err := opts.CurrentHead(projectRoot)
		if err == nil && strings.TrimSpace(current) != rp.GitHead && !opts.AllowHeadMoved {
			return nil, fmt.Errorf("%w: git HEAD moved (was %s, now %s); set AllowHeadMoved to override",
				ErrStaleCheckpoint, rp.GitHead, strings.TrimSpace(current))
		}
	}

	// An empty worktree path skips the check: the cycle didn't use a
	// per-cycle worktree.
	if rp.WorktreePath != "" && !opts.PathExists(rp.WorktreePath) {
		return nil, fmt.Errorf("%w: worktree %s no longer exists",
			ErrStaleCheckpoint, rp.WorktreePath)
	}

	return rp, nil
}

// ActivateResumeStatePath points this process's cycle-state resolution at
// rp's origin file when it differs from the current resolution, returning a
// restore func (a no-op when nothing needed to change).
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

// RunCycleFromPhase resumes an in-flight cycle from the given phase,
// skipping completedPhases and replaying the rest of the cycle. Unlike
// RunCycle it never allocates a cycle number; it shares RunCycle's storage
// lock and terminal closeout.
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

	var execution resumeExecution
	defer func() {
		var preserve, completedNormally bool
		if c := execution.closeout; c != nil {
			preserve, completedNormally = c.preserveWorktree, c.cycleCompletedNormally
		}
		o.teardownCycleWorktree(req.ProjectRoot, execution.cycleState.ActiveWorktree, preserve, completedNormally)
	}()

	stopLease := startRunLease(cs.WorkspacePath, cs.RunID, o.now, leaseRefreshInterval())
	defer stopLease()

	inputs := o.snapshotResumeInputs(&boot)
	cs = boot.cycleState
	envSnap := inputs.env
	ctxSnap := inputs.context
	result = inputs.result
	preResumeHEAD := inputs.preResumeHEAD

	execution = resumeExecution{
		orchestrator:    o,
		ctx:             ctx,
		request:         req,
		resumePoint:     resumePoint,
		state:           state,
		cycleState:      cs,
		cycle:           cycle,
		startPhase:      startPhase,
		envSnapshot:     envSnap,
		contextSnapshot: ctxSnap,
		initialResult:   result,
		preResumeHEAD:   preResumeHEAD,
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
	phaseBaseline map[string]bool,
) (PhaseResponse, error) {
	if o.reviewer == nil || resp.Verdict == VerdictSKIPPED {
		return resp, nil
	}
	reviewInput := func(response PhaseResponse) ReviewInput {
		in := ReviewInputFor(cs, phase, projectRoot)
		in.Response = response
		return in
	}
	if err := o.recoverPhaseLeak(ctx, phaseLeakScope{projectRoot: projectRoot, cycleState: cs, phase: phase, baseline: phaseBaseline}); err != nil {
		return resp, err
	}
	review := o.performEffectsAndReview(ctx, reviewInput(resp))
	maxCorrections := (&cycleRun{o: o, cs: cs, retryConfig: o.retryConfig}).correctionLimitFor(phase, o.retryConfig.ContractCorrectionRetries)
	for correction := 1; !review.Approve && correction <= maxCorrections; correction++ {
		req.CorrectionDirective = composeCorrection(correction, review.Reason, review.Remediation)
		o.emitGateCorrection(gateCorrection{
			origin: "Orchestrator.reviewResumedDeliverable", cycle: cycle, phase: phase, correction: correction, max: maxCorrections,
			rung: interaction.RungRedispatch, cli: req.ModelRoutingCLI, reason: review.Reason,
		})
		cancel := o.observer.Start(ctx, string(phase), req)
		corrected, err := runner.Run(ctx, req)
		if cancel != nil {
			cancel()
		}
		if err != nil {
			if isQuotaWall(err) {
				return corrected, fmt.Errorf("resume review gate: phase %s correction %d: %w", phase, correction, quotaWall{err})
			}
			return corrected, fmt.Errorf("resume review gate: phase %q correction %d dispatch failed: %w", phase, correction, err)
		}
		if !IsVerdict(corrected.Verdict) {
			return corrected, fmt.Errorf("resume review gate: phase %q correction %d returned non-canonical verdict %q", phase, correction, corrected.Verdict)
		}
		resp = corrected
		if err := o.recoverPhaseLeak(ctx, phaseLeakScope{projectRoot: projectRoot, cycleState: cs, phase: phase, baseline: phaseBaseline}); err != nil {
			return resp, err
		}
		// Correction output is a fresh worktree mutation. Normalize it before
		// re-running the reviewer so a newly sealed snapshot is final.
		o.normalizeBuildWorktree(ctx, phase, cs, projectRoot)
		review = o.performEffectsAndReview(ctx, reviewInput(resp))
	}
	if !review.Approve {
		return resp, fmt.Errorf("resume review gate: phase %q deliverable rejected after %d correction(s): %s", phase, maxCorrections, review.Reason)
	}
	return resp, nil
}

func defaultCurrentHead(projectRoot string) (string, error) {
	// Capture, not gitexec's HEAD/Output helper, is deliberate: callers depend
	// on the raw, untrimmed `git rev-parse HEAD` stdout, trailing newline
	// included.
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
