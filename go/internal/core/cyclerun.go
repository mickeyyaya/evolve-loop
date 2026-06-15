package core

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
)

// cyclerun.go — methods extracted from the RunCycle engine (orchestrator.go) to
// keep RunCycle a readable coordinator. Each extraction is behavior-preserving;
// the orchestrator's characterization tests are the safety net.

// finalizeCycle runs RunCycle's post-loop finalization (extracted verbatim,
// behavior-preserving): reclassify the final verdict against pre/post HEAD, warn
// loudly on a silent no-ship, record shipped throughput, decide worktree
// preservation, and persist the cycle-end state.
//
// It returns whether the worktree must be preserved — the caller's exit defer
// reads this AFTER finalizeCycle returns, so it MUST be threaded back to
// RunCycle's frame (the R2 late-visibility contract); a persist error preserves
// nothing extra here, the defer's !cycleCompletedNormally clause covers it.
func (o *Orchestrator) finalizeCycle(ctx context.Context, cs CycleState, cycle int, preCycleHEAD string, result *CycleResult, state *State) (preserveWorktree bool, err error) {
	postCycleHEAD, _ := o.gitHEAD()
	result.FinalVerdict = o.finalizeOutcome(result.FinalVerdict, result.RetroDecision, preCycleHEAD, postCycleHEAD)

	// Notice the silent no-ship (Fix C): the cycle ran phases but ended without
	// HEAD advancing and without an audit-advisory "would-have-blocked" record —
	// i.e. work may have been produced and then discarded with the worktree
	// (cycle-148: a genuine PASS mis-graded FAIL routed audit→retro→end). The
	// outcome label alone is advisory and easily missed in a batch summary, so
	// surface it loudly here. Not an error — some cycles legitimately produce no
	// change — but always worth an operator's eyes.
	if result.FinalVerdict == CycleOutcomeSkippedUnknown {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d ended without shipping (%s): phases ran but HEAD did not advance and no audit-advisory block was recorded — any worktree changes were discarded. Inspect %s (audit-report.md verdict + acs-verdict.json red_count).\n", cycle, CycleOutcomeSkippedUnknown, cs.WorkspacePath)
	}

	// R9.1: a shipped cycle's committed floors are observed throughput —
	// record them into the rolling window before the state write below
	// persists it (nil seam ⇒ byte-identical no-op).
	if o.throughputRecorder != nil && shippedOutcome(result.FinalVerdict, preCycleHEAD, postCycleHEAD) {
		o.throughputRecorder(state, cycle, cs.WorkspacePath)
	}

	// A completed cycle that FAILED its verdict keeps its worktree for salvage
	// (inbox preserve-worktree-on-verdict-fail). The exit defer prunes only when
	// !preserveWorktree, so the caller sets the flag from this return before
	// marking completion. This MUST stay AFTER finalizeOutcome above: it reads
	// the FINAL verdict, so a SKIPPED/SHIPPED_VIA_BUILD reclassification has
	// already happened — reading it earlier would preserve on a pre-reclassification
	// raw FAIL. L3 gc (internal/gc) reclaims preserved worktrees on retention;
	// `evolve cycle reset` / `evolve loop --resume` reclaim them explicitly.
	preserveWorktree = preserveOnVerdict(result.FinalVerdict)

	state.LastCycleNumber = cycle
	if perr := o.persistCycleEndState(ctx, *state); perr != nil {
		return preserveWorktree, fmt.Errorf("write state: %w", perr)
	}
	return preserveWorktree, nil
}

// cycleInit carries the resources RunCycle's setup produces and the rest of the
// cycle consumes: the read state, the freshly-built CycleState, the allocated
// cycle number, and the main-tree dirty baseline (subtracted by recoverBuildLeak
// so it only relocates paths the build introduced).
type cycleInit struct {
	state             State
	cs                CycleState
	cycle             int
	mainDirtyBaseline map[string]bool
}

// newCycleRun performs RunCycle's resource setup (extracted behavior-preserving):
// acquire the project lock (skipped under fleet mode), read state, allocate the
// cycle number, mint the run ID, build the CycleState, archive a polluted
// workspace, provision the source worktree, persist the cycle state, and start
// the run lease.
//
// Defer contract (R2 late-visibility): the four cleanup actions (lock release,
// run-ID clear, worktree cleanup, lease stop) MUST run in RunCycle's frame, not
// here. So newCycleRun returns a single `cleanup` closure that RunCycle defers;
// the closure runs the actions LIFO — exactly the order the original five inline
// defers fired (stopLease → worktree → runID-clear → release; the phase-timing
// defer RunCycle registers later still fires first). The worktree branch reads
// `preserve`/`completedNormally` as PARAMETERS so RunCycle passes its own locals'
// LATE values at defer-execution time, mirroring the inline defer's behavior.
//
// On any error AFTER a resource is acquired, newCycleRun runs the accumulated
// cleanups itself with the pre-mutation (false, false) flags — the same values
// the inline defers observed on an early `return` — and returns a nil closure.
func (o *Orchestrator) newCycleRun(ctx context.Context, req CycleRequest) (cycleInit, func(preserve, completedNormally bool), error) {
	// LIFO cleanup stack. Each entry takes the late-mutated worktree flags; all
	// but the worktree-cleanup entry ignore them.
	var stack []func(preserve, completedNormally bool)
	run := func(preserve, completedNormally bool) {
		for i := len(stack) - 1; i >= 0; i-- {
			stack[i](preserve, completedNormally)
		}
	}
	// failClean mirrors what the inline defers saw at an early return: the
	// worktree flags are still their initial false/false at every error path
	// in this block (preserveWorktree and cycleCompletedNormally are mutated
	// only later in RunCycle's loop).
	failClean := func() { run(false, false) }

	// ADR-0049 S6 / root-cause R1: under the fleet supervisor (EVOLVE_FLEET=1)
	// skip the whole-cycle global project lock (LOCK_NB) so M cycles run
	// concurrently instead of refusing each other. Safe because every shared
	// resource is now serialized by its OWN flock — state.json (UpdateState /
	// withStateLock, S2), the ledger chain (CA.1), the .evolve/ship.lock
	// integrator (S5) — and each cycle is isolated by its per-run worktree +
	// workspace with run-scoped ship reads (S3) and audit binding (S4). Default
	// off → the live sequential loop keeps the global lock, byte-identical.
	release := func() error { return nil }
	if !fleetMode(req.Env) {
		acquired, err := o.storage.AcquireLock(ctx)
		if err != nil {
			return cycleInit{}, nil, fmt.Errorf("acquire lock: %w", err)
		}
		release = acquired
	}
	stack = append(stack, func(_, _ bool) { _ = release() })

	state, err := o.storage.ReadState(ctx)
	if err != nil {
		failClean()
		return cycleInit{}, nil, fmt.Errorf("read state: %w", err)
	}
	// CA.4: mint the cycle number through the allocation lease when the
	// storage supports the serialized RMW (legacy +1 otherwise). A crashed
	// run burns its number; resume re-enters via RunCycleFromPhase with the
	// run record's cycle and never re-allocates.
	cycle, err := o.allocateCycle(ctx, &state)
	if err != nil {
		failClean()
		return cycleInit{}, nil, fmt.Errorf("allocate cycle: %w", err)
	}

	startedAt := o.now().UTC().Format(time.RFC3339)
	// IntentRequired is the gate for the start→intent vs start→scout
	// edge. Source priority: explicit Context["intent_required"]=="true"
	// from the caller > env EVOLVE_REQUIRE_INTENT=="1" > false. This
	// mirrors the bash dispatcher's check at run-cycle.sh:build_context.
	intentRequired := req.Context["intent_required"] == "true" ||
		envchain.BoolValue(req.Env["EVOLVE_REQUIRE_INTENT"], false)
	// CA.5: one ULID per run — persisted in the cycle state; the
	// construction-time stampingLedger stamps it on every ledger entry for
	// as long as it is the current id (cleared on every exit path).
	runID := MintRunID(o.now())
	o.currentRunID.Store(runID)
	stack = append(stack, func(_, _ bool) { o.currentRunID.Store("") })
	cs := CycleState{
		CycleID:        cycle,
		Phase:          string(PhaseStart),
		StartedAt:      startedAt,
		PhaseStartedAt: startedAt,
		WorkspacePath:  RunWorkspacePath(req.ProjectRoot, cycle),
		IntentRequired: intentRequired,
		RunID:          runID,
	}
	// Guard against workspace pollution from a prior killed attempt at
	// the same cycle number. If `<workspace>/` exists and has files,
	// rename to `<workspace>.polluted-<UTCnano>/` BEFORE any phase runs.
	// Without this, leftover scout-report.md / build-report.md from the
	// killed attempt cause Scout to short-circuit (read pre-existing
	// artifacts in seconds instead of redoing discovery) and steer
	// downstream phases via the OLD task selection.
	// Source incident: cycle-108 meta-loop attempts 1-4 (2026-05-26).
	// Opt-out via EVOLVE_DISABLE_WORKSPACE_GUARD=1 — used by tests that pre-seed
	// workspace files to simulate phase state, and by operators via the shell
	// (captured into req.Env from filterEvolveEnv(os.Environ()) at cycle launch,
	// cmd_cycle.go). ADR-0049 N9: read ONLY the per-cycle env SNAPSHOT, never live
	// os.Getenv — under concurrent fleet cycles a peer's env (or a mid-flight
	// mutation) must not flip this cycle's guard. The launch snapshot already
	// carries the operator's shell value, so this is behavior-preserving for the
	// live loop while restoring per-cycle isolation.
	guardDisabled := envchain.BoolValue(req.Env["EVOLVE_DISABLE_WORKSPACE_GUARD"], false)
	if !guardDisabled {
		if err := archivePollutedWorkspace(cs.WorkspacePath, o.now); err != nil {
			// Best-effort: WARN but don't block the cycle; the failure
			// mode it prevents is bad-data steering, not safety.
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN workspace archive failed: %v\n", err)
		}
	}
	// Full main-tree dirty baseline (tracked + untracked) captured BEFORE any
	// phase runs. recoverBuildLeak (cycle-160 / Option A) subtracts it so it only
	// relocates paths the build introduced, never the operator's pre-existing work.
	mainDirtyBaseline := porcelainDirtySet(ctx, req.ProjectRoot)
	// Provision the per-cycle source worktree (ADR-0027): tdd/build write code
	// here, isolated from the live tree. cs.ActiveWorktree gates source writes
	// in the role-gate and drives worktree-aware ship. Best-effort — on failure
	// the source phases are denied by the role-gate (loud, not silent). Cleaned
	// up on cycle exit (after ship has merged the worktree→main).
	// cs.WorktreeBaseSHA (persisted) is the worktree HEAD at creation == the
	// cycle base. After the build phase we soft-reset to it so a committing
	// builder's work becomes pending again (see normalizeWorktreeToBase + the
	// cycle-156 incident). Persisted in CycleState so the crash-resume path
	// can run the same normalize.
	// preserveWorktree (ADR-0039 §8, D10 fix): set when a ship-stage failure
	// is recorded and cleared only when a later ship attempt succeeds. While
	// set, the exit cleanup below SKIPS pruning so audited (possibly
	// uncommitted) work survives for recovery — `evolve loop --resume` or an
	// explicit `evolve cycle reset` reclaims it. Cycle 7 lost its entire
	// PASS work to this prune; cycle 12 survived only via operator snapshot.
	if wtPath, werr := o.worktree.Create(req.ProjectRoot, cycle); werr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN worktree provisioning failed (source phases will be blocked): %v\n", werr)
	} else {
		cs.ActiveWorktree = wtPath
		if base, _, berr := gitCapture(ctx, wtPath, "rev-parse", "HEAD"); berr == nil {
			cs.WorktreeBaseSHA = strings.TrimSpace(base)
		} else {
			// Fail loudly: an empty base disables the cycle-156 normalize, so a
			// committing builder's work would again be discarded by the audit —
			// the exact symptom Option C fixes. WARN rather than abort (the
			// source phases still run; normalize just degrades to a no-op).
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN worktree-normalize: rev-parse HEAD at worktree creation failed: %v (build-commit normalize disabled this cycle)\n", berr)
		}
		stack = append(stack, func(preserve, completedNormally bool) {
			if preserve || !completedNormally {
				fmt.Fprintf(os.Stderr, "[orchestrator] preserving worktree %s — cycle ended abnormally; recover via `evolve loop --resume` or reclaim with `evolve cycle reset`\n", wtPath)
				return
			}
			_ = o.worktree.Cleanup(req.ProjectRoot, wtPath)
		})
	}
	if err := o.storage.WriteCycleState(ctx, cs); err != nil {
		failClean()
		return cycleInit{}, nil, fmt.Errorf("init cycle-state: %w", err)
	}

	// ADR-0049 G16: write + heartbeat the per-run .lease so gc's liveness check
	// (runlease.Fresh) never reaps a concurrent fleet sibling's run dir mid-cycle.
	// startRunLease creates the run dir itself; no-op for worktree-less / test
	// cycles (empty WorkspacePath). Stopped on every exit (deferred).
	stopLease := startRunLease(cs.WorkspacePath, runID, o.now, leaseRefreshInterval())
	stack = append(stack, func(_, _ bool) { stopLease() })

	return cycleInit{
		state:             state,
		cs:                cs,
		cycle:             cycle,
		mainDirtyBaseline: mainDirtyBaseline,
	}, run, nil
}
