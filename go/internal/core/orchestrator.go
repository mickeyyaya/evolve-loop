package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/research"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/verdictcache"
)

// PhaseBoundaryCheckpointer is a package-level hook to write a checkpoint block
// at phase boundaries, set by the checkpoint package to avoid circular imports.
var PhaseBoundaryCheckpointer func(cs CycleState, projectRoot string, now time.Time) error

func wrapCycleLevelError(phase Phase, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrPhaseGateFailed) || errors.Is(err, ErrLedgerChainBroken) || errors.Is(err, ErrLockHeld) {
		return err
	}
	return &ErrCycleLevelFailure{Phase: string(phase), Cause: err}
}

// optionalInfraSkip reports whether a phase whose retries exhausted may
// degrade to WARN+advance instead of aborting the cycle (the Workstream-D
// intent documented on ErrArtifactTimeout; cycle-283). Three conditions, all
// required: the error is INFRA-shaped (artifact timeout / transient bridge —
// never integrity or logic failures), the phase is catalog-Optional, and the
// phase sits outside the resolved ship floor — so the skip can never weaken
// `ship ⇒ build ∧ audit ∧ tdd`. Ship is always mandatory by convention; its
// explicit guard is belt-and-suspenders against a misconfigured catalog
// (ship is not in the floor set, so the floor loop would not catch it).
func (o *Orchestrator) optionalInfraSkip(p Phase, err error) bool {
	if !errors.Is(err, ErrArtifactTimeout) && !isTransientBridgeError(err) {
		return false
	}
	if p == PhaseShip {
		return false
	}
	spec, ok := o.catalog.Get(string(p))
	if !ok || !spec.Optional {
		return false
	}
	name := string(p)
	for _, f := range o.resolvedShipFloor() {
		if name == f {
			return false
		}
	}
	return true
}

// PhaseMinter registers a minted phase config into a dispatchable runner. The
// orchestrator depends on this narrow port (not phaseregistrar directly) so
// core stays decoupled from specrunner/phaseregistrar; the composition root
// adapts the concrete Registrar to it. Register validates + clamps the config
// against the trust-kernel guardrails and returns the normalized spec + runner,
// or an error (out-of-envelope tier, disallowed CLI, invalid spec).
// Implementations MUST return a spec with Optional=true (a minted phase can
// never satisfy or displace the build→audit→ship floor); the orchestrator's
// transitionLegal gate independently rejects a non-optional candidate, so a
// non-conforming minter degrades to a refused edge rather than a kernel breach.
type PhaseMinter interface {
	Register(cfg phaseconfig.PhaseConfig) (phasespec.PhaseSpec, PhaseRunner, error)
}

// CycleRequest is the operator-facing input to RunCycle.
type CycleRequest struct {
	ProjectRoot string
	GoalHash    string
	// Env is propagated to every PhaseRequest.Env that runs in this
	// cycle. Phases consult it for CLI/model selection
	// (EVOLVE_CLI, EVOLVE_<PHASE>_MODEL, …). The orchestrator copies the
	// map so post-RunCycle operator mutation does not affect in-flight
	// or completed runs.
	Env map[string]string
	// Context seeds the PhaseRequest.Context every phase receives. Ship
	// requires Context["commit_message"]; Scout reads
	// Context["strategy"]. Copied like Env.
	Context map[string]string
}

// CycleResult summarises what RunCycle did.
type CycleResult struct {
	Cycle        int
	FinalVerdict string
	PhasesRun    []Phase
	// RetroDecision is the failure-adapter's verdict on the retro branch,
	// populated only when retro ran. Format: "<action>: <reason>".
	RetroDecision string
}

// Orchestrator drives one cycle through the state machine, calling a
// PhaseRunner per phase and appending ledger entries. It is pure: all
// I/O is delegated to the injected Storage and Ledger ports.
//
// This is the Phase 1 skeleton — guards, observer, budget enforcement
// land in Phase 2.
type Orchestrator struct {
	storage Storage
	ledger  Ledger
	runners map[Phase]PhaseRunner
	sm      *StateMachine
	now     func() time.Time
	// gitHEAD returns the current git HEAD SHA. Called once at cycle
	// start and once before finalizing the verdict so the orchestrator
	// can detect whether anything got committed during the cycle (e.g.
	// when the build phase invokes `evolve ship --class manual` inline).
	// Errors are swallowed and treated as "no movement detected" — the
	// outcome calculator falls back to SKIPPED_UNKNOWN.
	gitHEAD func() (string, error)

	// gitDirtyPaths returns the set of modified tracked paths in the main
	// repo's working directory (`git diff --name-only HEAD` in repoRoot).
	// Workstream B's tree-diff guard snapshots this before each source-
	// writing phase and compares after — any newly-dirty MAIN-tree path is a
	// leak that escaped the sandbox (each git worktree is a separate working
	// dir, so its writes don't show up here). Injected for tests.
	gitDirtyPaths func(ctx context.Context, repoRoot string) ([]string, error)

	// catalogRefresh optionally refreshes the live model catalog at cycle start
	// (WithCatalogRefresher). It owns its own staleness check (TTL) and is
	// best-effort: errors WARN and never block the cycle. nil ⇒ no auto-refresh
	// (the composition root wires the closure; core never imports modelcatalog).
	catalogRefresh func(ctx context.Context) error

	// worktree provisions/cleans the per-cycle source worktree (ADR-0027).
	// Default gitWorktree (real git); injected in tests via
	// WithWorktreeProvisioner so RunCycle runs without touching real git.
	worktree WorktreeProvisioner

	// cfg + strategy drive dynamic phase routing ("model proposes, kernel
	// disposes"). The zero value (Stage:Off, StaticPreset) reproduces the
	// legacy static-state-machine behavior byte-for-byte: routing is
	// computed only when the composition root opts in via WithRouting with
	// a non-Off stage. The orchestrator never reads a routing flag itself —
	// config.Load (the composition root) is the sole env/file reader.
	cfg      config.RoutingConfig
	strategy router.RoutingStrategy

	// planner produces the upfront whole-cycle plan (ADR-0024 §2). Optional:
	// nil ⇒ no advisor plan ⇒ the kernel floor falls back to the configurable
	// never-skip spine (fail-safe to static). Consulted once at cycle start,
	// only at Stage>=Advisory; its output is clamped to the integrity floor
	// before being threaded into every routing decision.
	planner router.Planner

	// catalog is the merged phase catalog (built-in + user overlays). It lets
	// the orchestrator accept and run user-defined phases on the dynamic-routing
	// path WITHOUT hardcoding them in the Phase enum / state machine. Empty (the
	// default) ⇒ only built-in phases exist ⇒ byte-identical legacy behavior.
	catalog phasespec.Catalog

	// registrar mints advisor-proposed phases at cycle start (Steps 11/12).
	// Nil (default) ⇒ MintPhases are ignored ⇒ byte-identical legacy behavior.
	// Set via WithRegistrar; the composition root adapts phaseregistrar.Registrar.
	registrar PhaseMinter

	// kb is the knowledge-base recall port (WS2): at plan time the orchestrator
	// looks up prior lessons matching the most recent failure and threads them
	// into the advisor's prompt (recall memory). Nil (default) ⇒ no recall is
	// added ⇒ byte-identical legacy behavior. Set via WithKB; the composition
	// root wires research.NewFileKB(research.SearchPathsFromEnv()).
	kb research.KB

	// shipFloor is the resolved integrity floor (WS4): the phases a plan reaching
	// ship MUST run. Empty (default) ⇒ router.DefaultShipFloor ({tdd,build,audit},
	// byte-identical legacy behavior). The composition root sets it from
	// policy.FloorPhases() when the user configured an explicit .evolve/policy.json
	// ship_floor (e.g. ["audit"] for the audit-only posture). The router self-seals
	// the non-removable evaluator regardless, so this can only relax build/tdd.
	shipFloor []string

	// reviewer adjudicates a finished phase's deliverable before the cycle
	// advances (Workstream E2). Nil ⇒ noopReviewer default ⇒ every non-error,
	// non-SKIPPED verdict is recorded as a success (pre-E2 behavior). Set via
	// WithReviewer; the deterministic default + future LLM reviewer
	// implementations share the DeliverableReviewer interface.
	reviewer DeliverableReviewer

	// observer is the per-phase stall detector (cycle-122 Fix 3 / ADR-0030).
	// Start is called once before each runner.Run; the returned cancel runs
	// once after. Nil ⇒ noopObserver default ⇒ byte-identical to the pre-
	// ADR-0030 cycle. Set via WithObserver; cmd_cycle.go wires the real
	// implementation when ObserverPolicy.Autospawn is enabled.
	observer Observer

	// failureAdviser is the ADR-0044 C3 LLM escalation tail consulted by
	// adviseOnUnclassifiedFailure (failure_hook.go) — only at
	// cfg.PhaseRecovery == StageEnforce, only for unclassified
	// artifact-timeout panes. Nil (default) ⇒ hook inert. Set via
	// WithFailureAdviser.
	failureAdviser FailureAdviser

	// contractVerifier is the ADR-0045 I2 breaker-neutral deliverable
	// re-check used by the correction ladder's salvage rung. Nil (default)
	// ⇒ the salvage rung gets zero budget and the ladder degrades to
	// redispatch-only — exactly the pre-I2 correction loop.
	contractVerifier ContractVerifier

	// throughputRecorder observes shipped cycles' coverage-floor counts for
	// the R9 triage-capacity window (throughput_hook.go). Nil (default) ⇒
	// no-op. Set via WithThroughputRecorder.
	throughputRecorder ThroughputRecorder

	// currentRunID holds the in-flight run's ULID (CA.5) as a string; the
	// construction-time stampingLedger reads it atomically on every Append.
	// Empty ⇒ no run in flight ⇒ entries are not stamped.
	currentRunID atomic.Value
}

// Option customizes an Orchestrator at construction (functional-options DI).
// Absent any option, the orchestrator runs in legacy Stage:Off mode.
type Option func(*Orchestrator)

// WithRouting injects the loaded routing config + the strategy selected once
// at the composition root. A nil strategy is ignored so the StaticPreset
// default stands; the orchestrator depends only on the RoutingStrategy
// interface, never on a mode conditional.
func WithRouting(cfg config.RoutingConfig, strategy router.RoutingStrategy) Option {
	return func(o *Orchestrator) {
		o.cfg = cfg
		if strategy != nil {
			o.strategy = strategy
		}
	}
}

// WithPlanner injects the whole-cycle phase planner (ADR-0024 §2 hybrid
// cadence). A nil planner is ignored so the no-plan default stands; the
// orchestrator consults it only at Stage>=Advisory and always clamps its
// output to the integrity floor — "model proposes, kernel disposes".
func WithPlanner(p router.Planner) Option {
	return func(o *Orchestrator) {
		if p != nil {
			o.planner = p
		}
	}
}

// WithCatalog injects the merged phase catalog so the orchestrator can accept
// and run user-defined (non-built-in) phases on the dynamic-routing path. The
// empty default keeps behavior byte-identical to the built-in-only pipeline.
func WithCatalog(cat phasespec.Catalog) Option {
	return func(o *Orchestrator) { o.catalog = cat }
}

// WithRegistrar injects the phase minter so the orchestrator can register
// advisor-proposed phases at cycle start (Steps 11/12). Nil is ignored, leaving
// the no-mint default (byte-identical legacy behavior).
func WithRegistrar(m PhaseMinter) Option {
	return func(o *Orchestrator) {
		if m != nil {
			o.registrar = m
		}
	}
}

// WithKB injects the knowledge-base recall port (WS2). Nil is ignored, leaving
// the no-recall default (byte-identical legacy behavior). The composition root
// wires research.NewFileKB(research.SearchPathsFromEnv()).
func WithKB(kb research.KB) Option {
	return func(o *Orchestrator) {
		if kb != nil {
			o.kb = kb
		}
	}
}

// WithShipFloor sets the resolved integrity floor (WS4) — the phases a plan
// reaching ship must run. Empty/nil is ignored, leaving the safe structural
// default (router.DefaultShipFloor). The composition root passes the user's
// policy.FloorPhases() result when an explicit ship_floor is configured.
func WithShipFloor(floor []string) Option {
	return func(o *Orchestrator) {
		if len(floor) > 0 {
			o.shipFloor = floor
		}
	}
}

// WithWorktreeProvisioner injects a worktree provisioner. Tests pass a fake to
// avoid real git; nil is ignored so the gitWorktree default stands.
func WithWorktreeProvisioner(p WorktreeProvisioner) Option {
	return func(o *Orchestrator) {
		if p != nil {
			o.worktree = p
		}
	}
}

// WithObserver injects a per-phase stall detector (cycle-122 Fix 3 / ADR-0030).
// The orchestrator calls observer.Start(...) before each phase's runner.Run
// and the returned cancel after — running a background watcher that emits
// stall_no_output events to the workspace when the subagent's stdout-log
// stops growing. A nil observer (default) keeps the noopObserver default,
// which is byte-identical to the pre-ADR-0030 cycle.
//
// cmd_cycle.go wires the real implementation via
// observer.NewCoreAdapter when ObserverPolicy.Autospawn is enabled.
func WithObserver(o Observer) Option {
	return func(orch *Orchestrator) {
		if o != nil {
			orch.observer = o
		}
	}
}

// WithCatalogRefresher injects a best-effort live-model-catalog refresh run at
// cycle start. The closure owns its TTL/staleness check; the orchestrator calls
// it once per cycle before any phase runs and only WARNs on error (never blocks).
func WithCatalogRefresher(fn func(ctx context.Context) error) Option {
	return func(o *Orchestrator) { o.catalogRefresh = fn }
}

// WithReviewer injects a per-phase deliverable reviewer (Workstream E2). The
// orchestrator calls reviewer.Review(...) after each phase's runner.Run returns
// a non-error, non-SKIPPED verdict, BEFORE the ledger append or
// CompletedPhases++. Approve=false aborts the cycle with the reviewer's Reason
// (no retry budget yet — that's a follow-up; see the WS-E plan). A nil reviewer
// keeps the noopReviewer default, which is byte-identical to the pre-E2 cycle.
func WithReviewer(r DeliverableReviewer) Option {
	return func(o *Orchestrator) {
		if r != nil {
			o.reviewer = r
		}
	}
}

// WithContractVerifier injects the breaker-neutral deliverable re-check the
// ADR-0045 I2 salvage rung verifies relocations with. Nil is ignored, leaving
// the redispatch-only ladder (byte-identical to the pre-I2 correction loop).
// cmd_cycle.go wires deliverable.NewVerifierWithCatalog beside the reviewer.
func WithContractVerifier(v ContractVerifier) Option {
	return func(o *Orchestrator) {
		if v != nil {
			o.contractVerifier = v
		}
	}
}

// WithGitDirtyPaths overrides the git-dirty-path seam for the tree-diff guard.
// Intended for tests that want to control exactly what paths the guard sees
// without a real git repo.
func WithGitDirtyPaths(fn func(ctx context.Context, repoRoot string) ([]string, error)) Option {
	return func(o *Orchestrator) {
		if fn != nil {
			o.gitDirtyPaths = fn
		}
	}
}

// NewOrchestrator wires the orchestrator with its dependencies. Routing stays
// off unless a WithRouting option supplies an enabled-stage config.
func NewOrchestrator(storage Storage, ledger Ledger, runners map[Phase]PhaseRunner, opts ...Option) *Orchestrator {
	o := &Orchestrator{
		storage:       storage,
		ledger:        ledger,
		runners:       runners,
		sm:            NewStateMachine(),
		now:           time.Now,
		gitHEAD:       defaultGitHEAD,
		gitDirtyPaths: defaultGitDirtyPaths,
		worktree:      gitWorktree{},
		strategy:      router.StaticPreset{},
		reviewer:      noopReviewer{}, // WS-E2: byte-identical default until WithReviewer is used
		observer:      noopObserver{}, // cycle-122 Fix 3 / ADR-0030: byte-identical default until WithObserver is used
	}
	for _, opt := range opts {
		opt(o)
	}
	// CA.5: the run-id stamping decorator wraps the (possibly option-
	// replaced) ledger exactly once at construction. The per-run identity
	// flows through the atomic currentRunID — RunCycle never mutates the
	// ledger field, so goroutine-spawning observers can read it race-free.
	o.ledger = stampingLedger{inner: o.ledger, runID: &o.currentRunID}
	return o
}

// archivePollutedWorkspace renames <workspace>/ to
// <workspace>.polluted-<UTCnano>/ when it exists and is non-empty.
// Returns nil for the empty-or-missing case (the cycle just runs in a
// fresh directory). Returns the underlying error only when stat/rename
// actually fails. Tests inject a deterministic clock via now.
func fleetMode(env map[string]string) bool {
	return envchain.BoolValue(env["EVOLVE_FLEET"], false)
}

// RunCycle drives one cycle from PhaseStart to PhaseEnd, returning a
// summary of what ran. The lock is acquired up front (except in fleet mode,
// see fleetMode) and released on every exit path. State is updated
// incrementally so a crash leaves an inspectable trail in .evolve/.
func (o *Orchestrator) RunCycle(ctx context.Context, req CycleRequest) (CycleResult, error) {
	// Resource setup (lock, state read, cycle allocation, run-ID mint, CycleState,
	// workspace-pollution guard, source worktree, cycle-state persist, run lease)
	// → newCycleRun. It returns a single cleanup closure carrying the four exit
	// actions (lock release, run-ID clear, worktree prune/preserve, lease stop);
	// RunCycle defers it in its OWN frame so the actions fire here at cycle exit,
	// LIFO, exactly as the original inline defers did. The worktree branch reads
	// cr.preserveWorktree/cr.cycleCompletedNormally at defer-execution time — the
	// cycleRun method object holds those late-mutated fields (R2 late-visibility).
	init, cleanup, err := o.newCycleRun(ctx, req)
	if err != nil {
		return CycleResult{}, err
	}
	// cycleRun method object: the ONE addressable home for the dispatch loop's
	// shared + loop-carried state, so the sub-methods' late mutations are visible
	// to the exit defers and the next iteration (pointer receivers throughout).
	cr := &cycleRun{
		o:                 o,
		ctx:               ctx,
		req:               req,
		cycle:             init.cycle,
		mainDirtyBaseline: init.mainDirtyBaseline,
		state:             init.state,
		cs:                init.cs,
		result:            CycleResult{Cycle: init.cycle, FinalVerdict: VerdictPASS},
		current:           PhaseStart,
		lastVerdict:       VerdictPASS,
		// scheduledNext "", routingSeq 0, recoveryDepth 0, preserveWorktree false,
		// cycleCompletedNormally false — zero-valued by construction.
	}
	// Cleanup defer registered FIRST (fires LAST, LIFO) and BEFORE planCycle —
	// byte-identical registration order to the inline version. Reads the LATE
	// field values at exit (R2 late-visibility): the single most important
	// state-promotion of the method-object refactor.
	defer func() { cleanup(cr.preserveWorktree, cr.cycleCompletedNormally) }()

	// Pre-loop planning (catalog refresh, per-cycle env/ctx snapshots, fleet
	// scope, challenge-token mint, pre-cycle HEAD capture, clamped whole-cycle
	// advisory plan) → planCycle. Outputs thread into every routing decision in
	// the dispatch loop below.
	plan := o.planCycle(ctx, req, cr.state, cr.cs, cr.cycle)
	cr.envSnap = plan.envSnap
	cr.ctxSnap = plan.ctxSnap
	cr.preCycleHEAD = plan.preCycleHEAD
	cr.benchedCLIs = plan.benchedCLIs
	cr.clampedPlan = plan.clampedPlan

	// Deferred write of phase-timing.json runs even when RunCycle returns an
	// error so partial timing data is preserved for operator inspection.
	// Registered SECOND (fires FIRST, LIFO); reads cr.phaseTimings live so the
	// grown slice header is observed.
	defer func() {
		if len(cr.phaseTimings) > 0 {
			writePhaseTimings(cr.cs.WorkspacePath, cr.phaseTimings)
		}
		// ADR-0045 I1: roll every per-phase interaction ledger (bridge
		// subprocess + orchestrator producers alike) into
		// interaction-summary.json. Best-effort, abort paths included —
		// an interaction that isn't recorded with its outcome doesn't exist.
		if werr := interaction.WriteRollup(cr.cs.WorkspacePath); werr != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN interaction-summary write: %v\n", werr)
		}
	}()
	// ADR-0048 Slice B (SHADOW): content-addressed audit-reuse probe. If this
	// cycle's worktree content already matches a prior audited verdict, the
	// tdd/build/audit pipeline COULD be skipped and the prior verdict carried
	// forward. Observe-only — logs the would-reuse and changes nothing (mirrors
	// the Slice A shadow precedent; the EVOLVE_VERDICT_CACHE enforce dial lands
	// with the enforce stage, after soak observation per ADR-0046 discipline).
	//
	// Probe location is pre-loop ON PURPOSE: it targets the "fast re-land" case
	// (the ADR's cycles 247-248 motivation) — a preserved/re-dispatched worktree
	// that still carries content a prior cycle audited PASS. A FRESH cycle's
	// worktree is a clean HEAD clone with no build changes yet, so it will not
	// match (correctly — there is nothing to reuse before any work runs). A
	// richer post-build probe (same-tree-already-audited within a normal cycle)
	// is an enforce-stage decision, deliberately out of the shadow increment.
	if cr.cs.ActiveWorktree != "" {
		if sha := worktreeContentSHA(ctx, cr.cs.ActiveWorktree); sha != "" {
			if e, ok := verdictcache.NewStore(req.ProjectRoot, o.now).Lookup(sha); ok {
				fmt.Fprintf(os.Stderr, "[verdict-cache SHADOW] worktree tree_sha=%s matched cycle=%d verdict=%s — would skip tdd/build/audit (ADR-0048 Slice B; enforce pending)\n", sha, e.Cycle, e.Verdict)
			}
		}
	}

	// Bounded loop guards against any transition-table cycle bug.
	// Labeled so the extracted sub-methods can signal loop termination
	// (loopBreak → `break OuterLoop`) from inside the switch ladder below —
	// a bare `break` there would exit the switch, not the loop (H15).
OuterLoop:
	for safety := 0; safety < 32; safety++ {
		// Static transition + dynamic-routing override + spine-integrity gate +
		// PhaseEnd termination → selectNext.
		next, act, serr := cr.selectNext()
		switch act {
		case loopAbort:
			return cr.result, serr
		case loopBreak:
			break OuterLoop
		}

		// Runner lookup + pre-phase state write + tree-diff snapshot + phase-request
		// build + the inner attempt loop (retries/backfill/optional-infra-skip/
		// ship-recovery) → dispatch. Produces the per-phase dispatchResult.
		dr, act, derr := cr.dispatch(next)
		switch act {
		case loopAbort:
			return cr.result, derr
		case loopContinue:
			continue
		}

		// Per-phase deliverable review gate + correction ladder + ship-preserve
		// clear + worktree-leak recovery + post-phase tree-diff guard → reviewAndGuard.
		if act, rerr := cr.reviewAndGuard(next, &dr); act == loopAbort {
			return cr.result, rerr
		}

		// End-of-iteration record + branch (success ledger, bindings, normalize,
		// CompletedPhases persist, checkpoint, outcome record + cursor advance,
		// retro/debugger non-verdict-driven branches) → recordAndBranch.
		switch act, berr := cr.recordAndBranch(next, dr); act {
		case loopAbort:
			return cr.result, berr
		case loopBreak:
			break OuterLoop
		}

		// WS2-S0 (ADR-0052): post-scout re-plan hook. Fires once per cycle after
		// scout's handoff has been recorded (recordAndBranch above) and BEFORE the
		// next selectNext — gated on the just-completed phase being scout. Firing
		// here (post-record, pre-select) is what keeps the re-plan from widening the
		// run-set or bypassing the spine gate. No-op until WS2-S3 wires the shadow
		// RePlan behind EVOLVE_ROUTER_REPLAN.
		if next == PhaseScout {
			cr.postScoutReplan()
		}
	}

	// Post-loop finalization (verdict reclassification, silent-no-ship warn,
	// throughput, worktree-preserve decision, state persist) → finalizeCycle.
	// preserveWorktree is threaded back so the exit defer (registered above)
	// observes it; cycleCompletedNormally is set only on a clean persist.
	preserve, ferr := o.finalizeCycle(ctx, cr.cs, cr.cycle, cr.preCycleHEAD, &cr.result, &cr.state)
	if preserve {
		cr.preserveWorktree = true
	}
	if ferr != nil {
		return cr.result, ferr
	}
	cr.cycleCompletedNormally = true
	return cr.result, nil
}
