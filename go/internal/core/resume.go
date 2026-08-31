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

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
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
// Unlike RunCycle, RunCycleFromPhase does NOT increment LastCycleNumber
// — it operates on the cycle that's already in flight. It also does
// NOT re-acquire the cycle lock (the caller already holds it, since the
// checkpoint was written under lock).
func (o *Orchestrator) RunCycleFromPhase(ctx context.Context, req CycleRequest, resumePoint *ResumePoint) (CycleResult, error) {
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

	state, err := o.storage.ReadState(ctx)
	if err != nil {
		return CycleResult{}, fmt.Errorf("read state: %w", err)
	}

	cs, err := o.storage.ReadCycleState(ctx)
	if err != nil {
		return CycleResult{}, fmt.Errorf("read cycle-state: %w", err)
	}
	resumeIdentity, err := authoritativeResumeIdentity(req.ProjectRoot, resumePoint.StatePath, cs.WorkspacePath)
	if err != nil {
		return CycleResult{}, err
	}
	resumeWorkspace := resumeIdentity.workspace
	hostCycle := cs.CycleID
	if state.LastCycleNumber > 0 {
		// state.json is host-owned and independent of the Builder-writable
		// cycle-state projection. Fleet checkpoints override this with the cycle
		// encoded in their run-directory path below.
		hostCycle = state.LastCycleNumber
	} else if hostCycle == 0 {
		hostCycle = resumePoint.CycleID
	}
	resumeCycle := hostCycle
	if resumeIdentity.fleet {
		resumeCycle = resumeIdentity.cycle
	}
	recoveryBinding := cs
	if recoveryBinding.CycleID == 0 {
		recoveryBinding.CycleID = resumeCycle
	}
	if newBase, recoverable, recoveryErr := explanationdocs.RecoverRebaseSplit(ctx, explanationBinding(req.ProjectRoot, recoveryBinding)); recoveryErr != nil {
		return CycleResult{}, fmt.Errorf("recover rebased Build checkpoint: %w", recoveryErr)
	} else if recoverable {
		cs.WorktreeBaseSHA = newBase
		if err := o.storage.WriteCycleState(ctx, cs); err != nil {
			return CycleResult{}, fmt.Errorf("persist recovered rebased Build checkpoint: %w", err)
		}
		startPhase = PhaseBuild
	}
	if err := requireResumeExplanationIdentity(req.ProjectRoot, resumeWorkspace, resumeCycle, cs, resumePoint.CycleID); err != nil {
		return CycleResult{}, err
	}
	if cs.CycleID != 0 && resumePoint.CycleID != 0 && cs.CycleID != resumePoint.CycleID {
		return CycleResult{}, fmt.Errorf("resume identity mismatch: cycle-state cycle %d does not match checkpoint cycle %d", cs.CycleID, resumePoint.CycleID)
	}
	if cs.ActiveWorktree != "" && resumePoint.WorktreePath != "" && filepath.Clean(cs.ActiveWorktree) != filepath.Clean(resumePoint.WorktreePath) {
		return CycleResult{}, fmt.Errorf("resume identity mismatch: cycle-state worktree %q does not match checkpoint worktree %q", cs.ActiveWorktree, resumePoint.WorktreePath)
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
	// SSOT IPC-protocol-allowed: parent -> child resume-mode handoff (writer).
	envSnap["EVOLVE_"+"RESUME_MODE"] = "1"
	ctxSnap := make(map[string]string, len(req.Context))
	for k, v := range req.Context {
		ctxSnap[k] = v
	}

	result := CycleResult{Cycle: cycle, FinalVerdict: VerdictPASS}
	mainDirtyBaseline := porcelainDirtySet(ctx, req.ProjectRoot)

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
		// Survey emission is unconditional (a resumed cycle that dispatched
		// nothing still has pre-crash outputs to account); the timings write
		// keeps its emptiness guard. cs.CompletedPhases carries the pre-crash
		// completions plus everything this resume appended.
		emitPhaseOutputsSignal(cs.WorkspacePath, cycle, cs.CompletedPhases,
			phasecontract.NewCatalogResolver(o.catalog.Get))
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
			// Cycle-637: rehydrate the transition kernel from the run's own plan
			// so a resumed cycle can transition OUT of an advisor-inserted phase
			// (invalid on the static spine) instead of dying "invalid phase".
			// Spine-valid phases keep o.sm.Next byte-identically.
			n, err := o.resolveResumeNext(cs, current, lastVerdict)
			if err != nil {
				return result, fmt.Errorf("transition from %s: %w", current, err)
			}
			next = n
		}
		if next == PhaseEnd {
			break
		}
		if next == PhaseBuild {
			if err := sealBuildExplanationContext(req.ProjectRoot, cs); err != nil {
				return result, fmt.Errorf("resume seal Build explanation context: %w", err)
			}
		}

		runner, ok := o.runners[next]
		if !ok {
			// ADR-0044 C1 (cycle-637): a stranded successor on RESUME is a
			// terminal disposition that must funnel through the recording
			// chokepoint — the cycle-635 resume died FAILED_UNEXPLAINED precisely
			// because this bare error escaped it. Record a synthetic outcome
			// carrying the abort_reason so the outcome is FAILED_EXPLAINED.
			noRunnerErr := fmt.Errorf("%w: no runner registered for phase %s", ErrPhaseInvalid, next)
			o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath,
				phaseOutcomeFrom(next, PhaseResponse{}, 0, noRunnerErr.Error(), ""))
			return result, noRunnerErr
		}

		cs.Phase = string(next)
		// Stamp the per-phase wall-clock start here too (mirrors
		// cyclerun_dispatch.go): the resume path is a first-class dispatch
		// surface, so a resumed phase's timing record must carry started_at —
		// a post-crash resume is exactly when latency evidence matters most.
		cs.PhaseStartedAt = o.now().UTC().Format(time.RFC3339)
		cs.ActiveAgent = string(next)
		if next == PhaseAudit {
			// Mirrors cyclerun_dispatch.go (resume-parity): a resumed audit
			// re-dispatch supersedes any prior attempt's diagnosed-FAIL
			// explanation — stale reasons must never mark a later FAIL as
			// diagnosed to the ADR-0072 floor.
			resetFloorFailReason(&cs, next)
		}
		if err := o.storage.WriteCycleState(ctx, cs); err != nil {
			return result, fmt.Errorf("write cycle-state pre-%s: %w", next, err)
		}

		// Resume-path parity for the audit-repair brief (review MEDIUM): the
		// budget half was already mirrored below via consumeAuditRepairGrant, but
		// without seeding HERE a cycle that crashed mid-repair burned an attempt
		// and rebuilt BLIND — the exact crash-resilience case the persisted
		// counter exists for. Same state-derived rule as the live loop.
		// NEXT, not current: at this point `current` is the phase that ran in the
		// PREVIOUS iteration (it is reassigned to next only at the bottom of the
		// loop). Using it meant the first TDD dispatch after a resumed repair
		// grant — Retro->TDD, the exact crash-resume case this exists for — saw
		// repairSeededPhase(PhaseRetro)==false and rebuilt BLIND. Every sibling
		// line in this block keys on next for the same reason.
		phaseCtx := seedAuditRepairContext(ctxSnap, next, cs)
		if next == PhaseAudit {
			cs.AuditRepairActive = false
		}
		phaseReq := PhaseRequest{
			Cycle:       cycle,
			ProjectRoot: req.ProjectRoot,
			Workspace:   cs.WorkspacePath,
			// CB.1: the resume path is a first-class dispatch surface and must
			// thread the persisted worktree like the RunCycle loop does — a
			// resumed phase with Worktree="" runs cwd=main-tree (cycle-280 class).
			Worktree:                        cs.ActiveWorktree,
			WorktreeBaseSHA:                 cs.WorktreeBaseSHA,
			ExplanationDocumentationVersion: cs.ExplanationDocumentationVersion,
			// CB.5: same rule for the persisted run identity (resume reuses
			// the run-record id, so session names stay run-scoped).
			RunID:         cs.RunID,
			GoalHash:      req.GoalHash,
			PreviousPhase: string(current),
			Env:           envSnap,
			Context:       phaseCtx,
		}
		if next != PhaseBuild {
			projectBuildExplanation(req.ProjectRoot, cs).apply(&phaseReq)
		}
		resp, err := runner.Run(ctx, phaseReq)
		if err != nil {
			phaseErr := fmt.Errorf("phase %s: %w", next, err)
			o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, 1, phaseErr.Error(), cs.PhaseStartedAt))
			return result, phaseErr
		}
		if !IsVerdict(resp.Verdict) {
			ferr := fmt.Errorf("phase %s returned non-canonical verdict %q", next, resp.Verdict)
			o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, 1, ferr.Error(), cs.PhaseStartedAt))
			return result, ferr
		}
		// Resume parity with reviewAndGuard: host normalization must finish
		// before Build's explanation is reviewed and sealed.
		o.normalizeBuildWorktree(ctx, next, cs)
		resp, err = o.reviewResumedDeliverable(ctx, req.ProjectRoot, cycle, cs, next, runner, phaseReq, resp, mainDirtyBaseline)
		if err != nil {
			o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, 1, err.Error(), cs.PhaseStartedAt))
			return result, err
		}
		if cs.ExplanationDocumentationVersion != 0 && next != PhaseBuild &&
			o.worktreePhase(next) && containsString(cs.CompletedPhases, string(PhaseBuild)) {
			requiresBuild, refreshErr := explanationdocs.RefreshResult(ctx, explanationBinding(req.ProjectRoot, cs))
			if refreshErr != nil {
				return result, fmt.Errorf("resume refresh Build explanation after %s: %w", next, refreshErr)
			}
			if requiresBuild {
				scheduledNext = PhaseBuild
			}
		}

		if err := o.ledger.Append(ctx, LedgerEntry{
			TS:       o.now().UTC().Format(time.RFC3339),
			Cycle:    cycle,
			Role:     string(next),
			Kind:     "phase",
			ExitCode: 0,
		}); err != nil {
			lerr := fmt.Errorf("ledger append for %s: %w", next, err)
			o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, 1, lerr.Error(), cs.PhaseStartedAt))
			return result, lerr
		}

		o.emitPhaseBindings(ctx, cycle, req.ProjectRoot, cs, next, resp.Verdict)
		cs.CompletedPhases = append(cs.CompletedPhases, string(next))
		if err := o.storage.WriteCycleState(ctx, cs); err != nil {
			werr := fmt.Errorf("write cycle-state post-%s: %w", next, err)
			o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, 1, werr.Error(), cs.PhaseStartedAt))
			return result, werr
		}

		// Cycle-802 resume-path parity (Task 2): identical floor-gated verdict
		// write to cyclerun_record.go — a resumed non-floor phase (a batch mid-
		// recovery is exactly where the storm recurred) cannot clobber a floor
		// PASS. CompletedPhases carries phases from BOTH the prior session and
		// this resume (next appended above), so floorAlreadyCompleted correctly
		// sees an audit that PASSed before the crash.
		o.recordFinalVerdict(&result, next, resp.Verdict, o.floorAlreadyCompleted(cs.CompletedPhases))
		// Resume-path parity for the floor-verdict failure-learning guard
		// (cyclerun_record.go): a resumed authoritative phase — audit is the one
		// the skills-drift storm recurred on, and a mid-batch recovery is exactly
		// where it recurred — whose in-process CI-parity gate overrides a narrative
		// PASS to FAIL (dispatch err==nil) must feed failure-learning HERE too, or
		// the storm class is silently reproduced for resumed cycles specifically.
		// Same single-source *Orchestrator primitive as the live loop.
		// Resume-path parity for the judgment-lesson recorder (judgment_lesson.go),
		// mirroring the floor-verdict guard below: a resumed judgment phase's FAIL
		// must leave the same lesson, or the objection is lost for resumed cycles
		// specifically. Same single-source *Orchestrator primitive.
		if resp.Verdict == VerdictFAIL {
			o.recordJudgmentLesson(ctx, cycle, cs.WorkspacePath, next, &state, resp.Diagnostics)
		}
		if resp.Verdict == VerdictFAIL && o.isAuthoritativePhase(next) {
			o.recordFloorVerdictFailure(ctx, req, cycle, next, &state, &cs, resp.Diagnostics)
		}
		o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, 1, "", cs.PhaseStartedAt))
		current = next
		lastVerdict = resp.Verdict

		// Resume-path parity for the audit-FAIL disposition (ADR-0093). Without
		// this branch the resume surface falls through to sm.Next(audit, FAIL) =
		// retro, so a resumed cycle could NEVER repair — and since retro is now
		// terminal, the retry the policy table grants would be silently
		// unreachable on exactly the surface that exists for recovery. The live
		// loop's branch is cyclerun_record.go; both call the same primitive.
		if current == PhaseAudit && resp.Verdict == VerdictFAIL {
			branch, reason, sysFail := o.decideAfterAuditFail(cs)
			consumeAuditRepairGrant(&cs, reason)
			if sysFail != nil && result.SystemFailure == nil {
				result.SystemFailure = sysFail
			}
			if branch != PhaseRetro {
				if !o.sm.CanTransition(PhaseAudit, branch) {
					return result, fmt.Errorf("audit→%s not allowed by state machine", branch)
				}
				scheduledNext = branch
			}
		}

		// History-branch gate (ADR-0058): the branch-entry CONDITION is lockstep
		// with recordAndBranch (both key on successorStrategy == history, which
		// owns the degrade). The branch BODY differs by design — resume takes the
		// deterministic decideAfterRetro, whereas the live loop additionally
		// routes via decideAfterRetroRouted at advisory stage.
		if o.successorStrategy(current) == phasespec.BranchingHistory {
			cs.FailedAt = state.FailedAt // S4 dossier non-progress counters (additive)
			branch, extraEnv, reason, sysFail := o.decideAfterRetro(cs, resp.Verdict, state.FailedAt)
			for k, v := range extraEnv {
				envSnap[k] = v
			}
			// Resume-path parity for the bookkeeping-regrade once-per-cycle bound
			// (cyclerun_record.go, same recurrence class as the floor-verdict
			// guard above): without consuming the slot HERE, a resumed cycle
			// whose re-audit fails bookkeeping-only again regrades forever
			// (bounded only by the resume safety counter — ~15 LLM dispatches).
			// The next pre-phase WriteCycleState persists the consumed slot.
			consumeBookkeepingRegradeGrant(&cs, reason)
			result.RetroDecision = reason
			// ADR-0072 S4: the Go floor is non-bypassable on the resume path too —
			// a floor category halts + escalates rather than looping as task-level.
			if sysFail != nil && result.SystemFailure == nil {
				result.SystemFailure = sysFail
			}
			if branch == PhaseEnd {
				break
			}
			if !o.sm.CanTransition(PhaseRetro, branch) {
				return result, fmt.Errorf("retro→%s not allowed by state machine", branch)
			}
			scheduledNext = branch
		}

		// The debugger signal-branch gate (ADR-0058 S3) is intentionally NOT
		// mirrored here. Per the ADR the debugger override is live-loop-only: of
		// the two record/resume override sites, resume duplicates only the retro
		// (history) override. A cycle resumed at debugger therefore terminates via
		// Next(debugger,_)→end rather than re-running decideAfterDebugger — the
		// unchanged pre-ADR behavior. Lifting that to resume is a separate slice,
		// not an S3 byte-identity change.
	}

	// Resume completed — preserve LastCycleNumber (already advanced when
	// the original cycle started; resume doesn't re-advance it).
	if err := o.storage.WriteState(ctx, state); err != nil {
		return result, fmt.Errorf("write state: %w", err)
	}
	return result, nil
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
