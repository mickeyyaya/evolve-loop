package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards/treediff"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// loopAction signals how the slim RunCycle driver proceeds after a dispatch-loop
// sub-method returns. err is non-nil ONLY when action == loopAbort.
type loopAction int

const (
	loopNext     loopAction = iota // proceed to the next sub-step in the iteration
	loopContinue                   // `continue` the outer loop now (optional-runner skip, ship-recovery, debugger fall-through)
	loopBreak                      // terminate the loop → fall to finalizeCycle (PhaseEnd, retro→End, debugger→End)
	loopAbort                      // `return cr.result, err` immediately
)

// cycleRun is the method object (Replace Method with Method Object) for
// RunCycle's dispatch loop. ONE addressable struct; every sub-method takes a
// *cycleRun receiver so late mutations (preserveWorktree, cs, the loop cursors)
// are visible to RunCycle's exit defers (the R2 late-visibility contract) and to
// the next loop iteration. Field grouping mirrors the original inline locals.
type cycleRun struct {
	// engine handles (immutable for the cycle)
	o   *Orchestrator
	ctx context.Context // same ctx the inline closure/dispatch captured

	// per-cycle constants (set once at construction, never reassigned)
	req               CycleRequest
	cycle             int
	mainDirtyBaseline map[string]bool
	envSnap           map[string]string     // reference type; MUTATED in-loop (retro extraEnv merge), same map across iterations
	ctxSnap           map[string]string     // reference type; MUTATED in-loop (ship_error_* keys), same map across iterations
	preCycleHEAD      string                // read post-loop by finalizeCycle
	benchedCLIs       []router.BenchedCLI   // CLI-health snapshot; read in selectNext Decide
	clampedPlan       *router.PhasePlan     // clamped whole-cycle plan; nil ⇒ static spine
	retryConfig       policy.RetryConfig    // resolved once at orchestrator construction
	workflowConfig    policy.WorkflowConfig // resolved once at orchestrator construction

	// heavily-mutated shared state (mutated by sub-methods, read post-loop)
	state        State              // &cr.state passed to recordFailureLearning + finalizeCycle
	cs           CycleState         // the ONE authoritative CycleState the loop drives
	result       CycleResult        // accumulating result; mutated via &cr.result; returned on every abort
	phaseTimings []phaseTimingEntry // appended via &cr.phaseTimings; read by RunCycle's exit defer (live header)

	// loop-carried state-machine cursor (produced end of iter N, consumed start of iter N+1)
	current       Phase  // SM cursor
	lastVerdict   string // loop-carried verdict
	scheduledNext Phase  // authoritative next-phase injection (retro/debugger/ship-recovery)
	routingSeq    int    // monotonic per-cycle routing-artifact counter; incremented in selectNext AND recordAndBranch
	recoveryDepth int    // bounds ship-error recovery to maxRecoveryDepth; persists across iterations
	replanDepth   int    // ADR-0052 WS2-S5: post-scout re-plans run this cycle; capped by cfg.RePlanMaxDepth (check-before-increment)

	// late-visibility exit-defer flags (R2 contract; highest hazard)
	preserveWorktree       bool // set on ship-error, cleared on PASS ship, OR'd post-loop; read by RunCycle's cleanup defer at exit
	cycleCompletedNormally bool // set true only post-loop; read by the same cleanup defer at exit
}

// dispatchResult carries the per-phase locals dispatch produces and
// reviewAndGuard/recordAndBranch consume. These are PER-ITERATION values, NOT
// cycleRun fields (each iteration re-derives them).
type dispatchResult struct {
	resp           PhaseResponse   // runner result; resp.Verdict → result.FinalVerdict + lastVerdict
	attemptCount   int             // attempt-loop count; read by phaseOutcomeFrom at the record sites
	phaseWorktree  string          // cs.ActiveWorktree snapshot; ReviewInput.Worktree + correction directives
	treeGuard      *treediff.Guard // pre-phase guard; consumed by the post-phase tree-diff check
	beforeDirty    []string        // pre-phase dirty snapshot
	snapshotFailed bool            // pre-phase snapshot failed
	runner         PhaseRunner     // resolved runner; reviewAndGuard re-dispatches it in the correction ladder
	phaseReq       PhaseRequest    // the phase request; reviewAndGuard mutates CorrectionDirective for re-dispatch
}

// recordFailureLearning replaces RunCycle's inline closure: it builds the
// failureLearningRequest from cr's fields, preserving the exact pointer targets
// (&cr.state, &cr.cs, &cr.result, &cr.phaseTimings) the closure captured.
func (cr *cycleRun) recordFailureLearning(failed Phase, failErr error, attempt int) {
	cr.o.recordFailureLearning(cr.ctx, failureLearningRequest{
		CycleRequest: cr.req,
		Cycle:        cr.cycle,
		Failed:       failed,
		Err:          failErr,
		Attempt:      attempt,
		State:        &cr.state,
		CycleState:   &cr.cs,
		Context:      cr.ctxSnap,
		Env:          cr.envSnap,
		Result:       &cr.result,
		Timings:      &cr.phaseTimings,
	})
}

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
	// from the caller > policy WorkflowConfig.PhaseEnables["intent"]=="on" > false.
	intentRequired := req.Context["intent_required"] == "true" ||
		o.workflowConfig.PhaseEnables["intent"] == "on"
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

// cyclePlan carries the pre-loop planning outputs the dispatch loop threads into
// every routing decision: the per-cycle env/ctx snapshots, the HEAD captured
// before any phase ran, the CLI-health snapshot, and the clamped whole-cycle plan
// (nil ⇒ routing falls back to the static spine).
type cyclePlan struct {
	envSnap      map[string]string
	ctxSnap      map[string]string
	preCycleHEAD string
	benchedCLIs  []router.BenchedCLI
	clampedPlan  *router.PhasePlan
}

// advisorPlanInput builds the RouteInput for a whole-cycle advisor decision —
// shared by the initial Plan (current=start, empty signals) and the post-scout
// RePlan (current=scout, measured signals), so both reason from the SAME goal
// text, recall memory, catalog, carryover, and CLI-health bench. The two callers
// differ ONLY in current + signals (research P1/P2: identical context, one extra
// signal-conditioned call). Single source for the RouteInput shape — a field
// added here reaches both decisions, so they can never silently diverge.
func (o *Orchestrator) advisorPlanInput(ctx context.Context, current string, signals router.RoutingSignals, req CycleRequest, state State, cs CycleState, cycle int, env map[string]string, benchedCLIs []router.BenchedCLI) router.RouteInput {
	// WS2 recall memory: the most recent failure's reason + matching KB lessons,
	// so the advisor plans WITH the benefit of what went wrong before. No-op when
	// no KB is wired or no failure history.
	lastReason, lessons := o.recallForPlan(ctx, state.FailedAt)
	return router.RouteInput{
		Current: current,
		Signals: signals,
		Cfg:     o.cfg,
		Now:     o.now(),
		// The goal TEXT (Context["goal"] — the same key the dispatcher sets and
		// Scout reads; NOT Context["strategy"], the strategy MODE) lets the advisor
		// reason about WHAT the cycle is for. A nil/absent map key is safe (empty ⇒
		// no Goal section).
		Workspace:      cs.WorkspacePath,
		ProjectRoot:    req.ProjectRoot,
		Cycle:          cycle,
		Env:            env,
		LastReason:     lastReason,
		Lessons:        lessons,
		Catalog:        phaseCardsFromCatalog(o.catalog),
		GoalText:       req.Context["goal"],
		CarryoverTodos: carryoverTodosForAdvisor(state.CarryoverTodos),
		BenchedCLIs:    benchedCLIs,
		IntentRequired: cs.IntentRequired,
		PSMASEnabled:   o.workflowConfig.PSMASEnabled,
	}
}

// planCycle runs RunCycle's pre-loop planning (extracted behavior-preserving):
// refresh the live model catalog, take the per-cycle env/ctx snapshots, surface
// the fleet scope, mint the challenge token, capture pre-cycle HEAD, and compute
// the clamped whole-cycle advisory plan (registering any advisor-minted phases).
// All steps are best-effort (WARN, never block); it returns no error.
func (o *Orchestrator) planCycle(ctx context.Context, req CycleRequest, state State, cs CycleState, cycle int) cyclePlan {
	// Cycle-start live-model-catalog refresh (TTL-gated inside the closure).
	// Best-effort: a slow/failed refresh WARNs and never blocks the cycle.
	if o.catalogRefresh != nil {
		if err := o.catalogRefresh(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN model-catalog refresh failed: %v\n", err)
		}
	}

	// One snapshot per cycle — operator mutation post-call must not
	// retroactively change what phases saw.
	envSnap := make(map[string]string, len(req.Env))
	for k, v := range req.Env {
		envSnap[k] = v
	}
	ctxSnap := make(map[string]string, len(req.Context))
	for k, v := range req.Context {
		ctxSnap[k] = v
	}

	// ADR-0049 E: when `evolve fleet --plan` launched this cycle, EVOLVE_FLEET_SCOPE
	// carries its assigned (disjoint) task IDs. Surface it to triage via
	// Context["fleet_scope"] so the cycle selects ONLY its subset and concurrent
	// cycles never pick work touching the same files. Read from the env SNAPSHOT
	// (not live os.Getenv) so it stays per-cycle. Empty/unset ⇒ legacy behavior.
	if scope := envSnap["EVOLVE_FLEET_SCOPE"]; scope != "" {
		ctxSnap["fleet_scope"] = scope
	}

	// PR 6 (cycle-135 followup): mint the cycle's challenge token here —
	// ONCE per cycle, at orchestrator start, BEFORE any phase runs. Surface
	// it to every phase via Context["challengeToken"] (scout's ComposePrompt
	// reads it at scout.go:64) AND persist it to <workspace>/challenge-
	// token.txt so the agent-templates.md PR 5 fallback source is populated.
	// Pre-PR-6, no Go code injected the token; scout invented its own
	// (cycle 134 audit C1: "no-token-manual-run-cycle-134"; cycle 135 audit
	// C1: scout minted `59576594e2e8d5c3` instead of using `5b96ecb69a0c848f`
	// from challenge-token.txt). The mint is the same 8-byte-hex shape as
	// bridge.defaultChallengeToken so post-cycle ledger entries are
	// indistinguishable from the bridge-minted ones used pre-cycle-135.
	if _, alreadySet := ctxSnap["challengeToken"]; !alreadySet {
		var tokBytes [8]byte
		if _, err := rand.Read(tokBytes[:]); err == nil {
			tok := hex.EncodeToString(tokBytes[:])
			ctxSnap["challengeToken"] = tok
			// Best-effort workspace write — phase agents per agent-templates.md
			// PR 5 read this as fallback source #2 when inputs.challengeToken
			// is empty. Failure is logged but not fatal (the Context path is
			// the primary route; phases that can't read the file just rely on
			// Context).
			_ = os.MkdirAll(cs.WorkspacePath, 0o755)
			if err := os.WriteFile(filepath.Join(cs.WorkspacePath, "challenge-token.txt"), []byte(tok+"\n"), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN challenge-token.txt write failed: %v (Context route still works)\n", err)
			}
		} else {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN challenge token mint failed: %v (phase agents will fall back to their own protocol)\n", err)
		}
	}

	// Capture HEAD before any phase so finalizeOutcome can detect mid-cycle commits.
	preCycleHEAD, _ := o.gitHEAD()

	// Upfront whole-cycle plan (ADR-0024 §2). At Stage>=Advisory with a planner,
	// ask the advisor once which phases to run, CLAMP the answer to the integrity
	// floor (ship⇒build∧audit∧tdd), persist it, and thread the clamped plan into
	// every routing decision below. The clamp is the non-bypassable kernel floor:
	// it can only COMPLETE the ship-chain, never weaken it, so a hallucinated or
	// adversarial plan cannot reach ship without a real build+audit. Any
	// failure leaves clampedPlan nil ⇒ routing falls back to the configurable
	// never-skip spine (fail-safe to static). Below Advisory, no plan is computed.
	// This is the SINGLE gate for the upfront plan: Stage>=Advisory (the advisor
	// drives) AND Mode==DynamicLLM (static mode makes no LLM calls) AND a planner
	// is wired. The composition root passes WithPlanner unconditionally; the
	// Mode check lives here so the invariant ("LLM plan iff DynamicLLM+Advisory")
	// has one source of truth rather than two gates that could drift.
	// CLI-health snapshot, taken ONCE at cycle start and threaded to both the
	// whole-cycle plan input and every per-transition Decide: the advisor and
	// the dispatcher must reason from the SAME bench state (review H2 — two
	// reads could diverge when a bench expires mid-planning).
	benchedCLIs := benchedCLIsForRouting(req.ProjectRoot)
	var clampedPlan *router.PhasePlan
	if o.cfg.Stage >= config.StageAdvisory && o.cfg.Mode == config.ModeDynamicLLM && o.planner != nil {
		// The initial plan runs with EMPTY signals (no handoffs exist yet at cycle
		// start); the post-scout RePlan (WS2-S3) calls the SAME builder with
		// current="scout" + measured signals, so both reason from the same goal,
		// recall, catalog, carryover, and bench — one signal-conditioned call apart.
		planIn := o.advisorPlanInput(ctx, string(PhaseStart), router.RoutingSignals{}, req, state, cs, cycle, envSnap, benchedCLIs)
		// ClampPlanToFloorWith's tddPinned reads planIn.Signals, empty here (no
		// handoffs yet) — cycle_size!="trivial" evaluates true, so tdd is pinned on
		// the conservative (more-mandatory) side at plan time. The floor is the
		// user-resolved set (WS4) or the safe default; the router self-seals the
		// non-removable evaluator regardless.
		if raw, perr := o.planner.Plan(planIn); perr != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase advisor Plan failed (degrading to static spine): %v\n", perr)
		} else if raw != nil {
			// WS2-S2: record the WS2-S1 structural validation of the advisor's RAW
			// plan (pre-clamp, so the advisor's intent is visible) as standalone
			// telemetry. Report-only — it never alters the plan; the clamp below
			// remains the sole disposer.
			o.recordPlanRejections(ctx, cycle, cs, router.ValidatePlan(planIn, raw))
			var clamps []router.Clamp
			clampedPlan, clamps = router.ClampPlanToFloorWith(planIn, raw, o.resolvedShipFloor(), cs.IntentRequired)
			o.recordPhasePlan(ctx, cycle, cs, clampedPlan, clamps)
			// Register advisor-minted phases (Steps 11/12) into runners +
			// catalog + routing BEFORE the dispatch loop, so a minted phase the
			// plan selected is dispatchable + routable through the same path as a
			// built-in. The trust-kernel clamp is enforced inside the registrar.
			o.registerMintedPhases(clampedPlan)
		}
	}

	return cyclePlan{
		envSnap:      envSnap,
		ctxSnap:      ctxSnap,
		preCycleHEAD: preCycleHEAD,
		benchedCLIs:  benchedCLIs,
		clampedPlan:  clampedPlan,
	}
}
