package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/backfill"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards/treediff"
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
	// implementation when EVOLVE_OBSERVER_AUTOSPAWN != "0" (default 1).
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
// observer.NewCoreAdapter when EVOLVE_OBSERVER_AUTOSPAWN != "0" (default 1).
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
			return CycleResult{}, fmt.Errorf("acquire lock: %w", err)
		}
		release = acquired
	}
	defer func() { _ = release() }()

	state, err := o.storage.ReadState(ctx)
	if err != nil {
		return CycleResult{}, fmt.Errorf("read state: %w", err)
	}
	// CA.4: mint the cycle number through the allocation lease when the
	// storage supports the serialized RMW (legacy +1 otherwise). A crashed
	// run burns its number; resume re-enters via RunCycleFromPhase with the
	// run record's cycle and never re-allocates.
	cycle, err := o.allocateCycle(ctx, &state)
	if err != nil {
		return CycleResult{}, fmt.Errorf("allocate cycle: %w", err)
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
	defer o.currentRunID.Store("")
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
	preserveWorktree := false
	// Full main-tree dirty baseline (tracked + untracked) captured BEFORE any
	// phase runs. recoverBuildLeak (cycle-160 / Option A) subtracts it so it only
	// relocates paths the build introduced, never the operator's pre-existing work.
	mainDirtyBaseline := porcelainDirtySet(ctx, req.ProjectRoot)
	cycleCompletedNormally := false
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
		defer func() {
			if preserveWorktree || !cycleCompletedNormally {
				fmt.Fprintf(os.Stderr, "[orchestrator] preserving worktree %s — cycle ended abnormally; recover via `evolve loop --resume` or reclaim with `evolve cycle reset`\n", wtPath)
				return
			}
			_ = o.worktree.Cleanup(req.ProjectRoot, wtPath)
		}()
	}
	if err := o.storage.WriteCycleState(ctx, cs); err != nil {
		return CycleResult{}, fmt.Errorf("init cycle-state: %w", err)
	}

	// ADR-0049 G16: write + heartbeat the per-run .lease so gc's liveness check
	// (runlease.Fresh) never reaps a concurrent fleet sibling's run dir mid-cycle.
	// startRunLease creates the run dir itself; no-op for worktree-less / test
	// cycles (empty WorkspacePath). Stopped on every exit (deferred).
	stopLease := startRunLease(cs.WorkspacePath, runID, o.now, leaseRefreshInterval())
	defer stopLease()

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
		// WS2 recall memory: thread the most recent failure's reason + matching KB
		// lessons into the plan prompt so the advisor plans WITH the benefit of
		// what went wrong before. No-op when no KB is wired or no failure history.
		lastReason, lessons := o.recallForPlan(ctx, state.FailedAt)
		planIn := router.RouteInput{
			Current:     string(PhaseStart),
			Signals:     router.RoutingSignals{}, // no handoffs exist yet at cycle start
			Cfg:         o.cfg,
			Now:         o.now(),
			Workspace:   cs.WorkspacePath,
			ProjectRoot: req.ProjectRoot,
			Cycle:       cycle,
			Env:         envSnap,
			LastReason:  lastReason,
			Lessons:     lessons,
			Catalog:     phaseCardsFromCatalog(o.catalog),
			// The goal TEXT (Context["goal"] — the same key the dispatcher sets and
			// Scout reads; NOT Context["strategy"], which is the strategy MODE like
			// "balanced"/"harden") lets the advisor reason about WHAT the cycle is for
			// — selecting a design phase or minting when the work warrants it, instead
			// of planning blind. Reading a nil/absent map key is safe (empty ⇒ no Goal
			// section in the prompt).
			GoalText:       req.Context["goal"],
			CarryoverTodos: carryoverTodosForAdvisor(state.CarryoverTodos),
			BenchedCLIs:    benchedCLIs,
			IntentRequired: cs.IntentRequired,
			PSMASEnabled:   envchain.BoolValue(envSnap["EVOLVE_PSMAS_SKIP"], false),
		}
		// ClampPlanToFloorWith's tddPinned reads planIn.Signals, empty here (no
		// handoffs yet) — cycle_size!="trivial" evaluates true, so tdd is pinned on
		// the conservative (more-mandatory) side at plan time. The floor is the
		// user-resolved set (WS4) or the safe default; the router self-seals the
		// non-removable evaluator regardless.
		if raw, perr := o.planner.Plan(planIn); perr != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase advisor Plan failed (degrading to static spine): %v\n", perr)
		} else if raw != nil {
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

	result := CycleResult{Cycle: cycle, FinalVerdict: VerdictPASS}
	var phaseTimings []phaseTimingEntry
	// Deferred write of phase-timing.json runs even when RunCycle returns an
	// error so partial timing data is preserved for operator inspection.
	defer func() {
		if len(phaseTimings) > 0 {
			writePhaseTimings(cs.WorkspacePath, phaseTimings)
		}
		// ADR-0045 I1: roll every per-phase interaction ledger (bridge
		// subprocess + orchestrator producers alike) into
		// interaction-summary.json. Best-effort, abort paths included —
		// an interaction that isn't recorded with its outcome doesn't exist.
		if werr := interaction.WriteRollup(cs.WorkspacePath); werr != nil {
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
	if cs.ActiveWorktree != "" {
		if sha := worktreeContentSHA(ctx, cs.ActiveWorktree); sha != "" {
			if e, ok := verdictcache.NewStore(req.ProjectRoot, o.now).Lookup(sha); ok {
				fmt.Fprintf(os.Stderr, "[verdict-cache SHADOW] worktree tree_sha=%s matched cycle=%d verdict=%s — would skip tdd/build/audit (ADR-0048 Slice B; enforce pending)\n", sha, e.Cycle, e.Verdict)
			}
		}
	}

	current := PhaseStart
	lastVerdict := VerdictPASS
	// scheduledNext, when non-empty, overrides the state machine for
	// the next iteration. Set by the retro branch to inject the
	// failure-adapter's decision.
	var scheduledNext Phase
	// routingSeq names the per-cycle routing-decision artifacts
	// (routing-decision-<seq>.json). Incremented only when routing runs.
	routingSeq := 0
	// recoveryDepth bounds advisor-driven ship-error recovery across the whole
	// cycle (maxRecoveryDepth). Persists across loop iterations.
	recoveryDepth := 0
	recordFailureLearning := func(failed Phase, failErr error, attempt int) {
		o.recordFailureLearning(ctx, failureLearningRequest{
			CycleRequest: req,
			Cycle:        cycle,
			Failed:       failed,
			Err:          failErr,
			Attempt:      attempt,
			State:        &state,
			CycleState:   &cs,
			Context:      ctxSnap,
			Env:          envSnap,
			Result:       &result,
			Timings:      &phaseTimings,
		})
	}

	// Bounded loop guards against any transition-table cycle bug.
	for safety := 0; safety < 32; safety++ {
		var next Phase
		// fromSchedule marks an iteration whose `next` came from scheduledNext —
		// an authoritative injection by the retro branch, the ship-error recovery
		// seam, or the debugger decision. The dynamic-routing override
		// (enforceNext) must NOT second-guess such a transition, so it is gated on
		// !fromSchedule (generalizing the prior current!=PhaseRetro guard).
		fromSchedule := false
		switch {
		case scheduledNext != "":
			next = scheduledNext
			scheduledNext = ""
			fromSchedule = true
		case current == PhaseStart:
			// First edge is gated by intent_required, not by verdict.
			next = o.sm.NextFromStart(cs.IntentRequired)
		case !current.IsValid():
			// current is a user-defined phase (only reachable when dynamic
			// routing selected it). The static successor is simply the next
			// entry in the configured order; the routing block below refines it.
			next = o.nextInOrder(current)
		default:
			n, err := o.sm.Next(current, lastVerdict)
			if err != nil {
				return result, fmt.Errorf("transition from %s: %w", current, err)
			}
			next = n
		}

		// Dynamic routing (shadow → advisory → enforce). Stage:Off — the
		// default — leaves the static state machine fully in control: no
		// digest, no ledger entry, byte-identical to legacy. When enabled,
		// digest the completed handoffs, ask the Strategy for a decision,
		// record it forensically, and — at Advisory and above — let the clamped
		// decision override the static successor, re-validated against the
		// legality oracle (CanTransition) AND the artifact-backed spine gate
		// (SpineSatisfiedUpTo). Retro keeps its failure-adapter shim
		// (decideAfterRetro) while routing is bedded in, so routing never
		// overrides the retro branch. The configurable mandatory set
		// (cfg.Mandatory) decides which phases are never-skip; the integrity
		// floor (ClampPlanToFloor, applied to the upfront plan) decides what ship
		// still requires regardless of how small the operator makes that set.
		if o.cfg.Stage != config.StageOff {
			routingSeq++
			signals, _ := router.Digest(cs.WorkspacePath, cs.CompletedPhases)
			dec := o.strategy.Decide(router.RouteInput{
				Current:   string(current),
				Verdict:   lastVerdict,
				Signals:   signals,
				History:   entriesFromRecords(state.FailedAt),
				Cfg:       o.cfg,
				Completed: cs.CompletedPhases,
				Strict:    envchain.BoolValue(envSnap["EVOLVE_STRICT_AUDIT"], false),
				Now:       o.now(),
				// Proposer context (DynamicLLM only; ignored by pure Route).
				Workspace:   cs.WorkspacePath,
				ProjectRoot: req.ProjectRoot,
				Cycle:       cycle,
				Env:         envSnap,
				BenchedCLIs: benchedCLIs,
				// Clamped whole-cycle plan (Stage>=Advisory). nil below Advisory
				// or on planner failure ⇒ shouldRun runs the legacy trigger path.
				Plan:           clampedPlan,
				IntentRequired: cs.IntentRequired,
				PSMASEnabled:   envchain.BoolValue(envSnap["EVOLVE_PSMAS_SKIP"], false),
			})
			if o.cfg.Stage >= config.StageAdvisory && !fromSchedule {
				if forced, ok := o.enforceNext(current, next, signals, dec, planRunsShip(clampedPlan)); ok {
					next = forced
				}
				// Full spine-integrity check on the SELECTED next (static OR
				// override). R5 (cycle-283 fix): the gate now fails CLOSED at
				// EVOLVE_PHASE_RECOVERY=enforce when the absence is CLEAN —
				// Digest distinguishes a transient read miss (DigestDegraded)
				// from a genuine gap, which was the original fail-open
				// rationale. Sequence: re-digest once (the artifact may have
				// landed between the routing digest and this check); a
				// still-unsatisfied spine with a degraded digest, or any miss
				// below enforce, keeps the loud-WARN fail-open (shadow =
				// byte-compatible until the R8.5 dial flip); a clean absence
				// at enforce aborts FAILED-EXPLAINED with the worktree
				// preserved. The operator waiver stays cfg.Mandatory
				// (isConfiguredMandatory) — no new escape hatch.
				if next != PhaseEnd && !o.sm.SpineSatisfiedUpTo(next, signals, o.cfg) {
					// The re-digest runs only in this already-anomalous branch
					// (zero cost on the happy path). A non-nil digest ERROR
					// means we cannot even establish what is absent — that is
					// never a "clean absence", so it fails open like a
					// degraded read (review F6: a missing workspace must not
					// masquerade as a spine gap at enforce).
					fresh, derr := router.Digest(cs.WorkspacePath, cs.CompletedPhases)
					cleanAbsence := derr == nil && len(fresh.DigestDegraded) == 0
					switch {
					case derr == nil && o.sm.SpineSatisfiedUpTo(next, fresh, o.cfg):
						// Transient: the handoff appeared on re-read. Proceed.
						// (dec was decided from the stale signals — diagnostic
						// record only; the gate, not Decide, owns blocking.)
					case o.cfg.PhaseRecovery == config.StageEnforce && cleanAbsence:
						spineErr := fmt.Errorf("spine gate: next=%s blocked — a mandatory predecessor's handoff artifact is missing (clean absence, fail-closed; cycle-283 class)", next)
						o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, PhaseResponse{Phase: string(next)}, 0, spineErr.Error()))
						recordFailureLearning(next, spineErr, 1)
						return result, spineErr
					default:
						// Two fail-open sources, deliberately identical in
						// behavior: (a) dial below enforce (shadow default —
						// byte-compatible until the R8.5 flip), (b) the
						// absence is NOT clean (degraded read or digest
						// error). The reason string tells them apart.
						dec.Clamps = append(dec.Clamps, router.Clamp{
							Rule:     "spine-unsatisfied-warn",
							Proposed: string(next),
							Forced:   string(next),
						})
						reason := "would-block at enforce"
						switch {
						case derr != nil:
							reason = "re-digest error: " + derr.Error()
						case len(fresh.DigestDegraded) > 0:
							reason = "digest degraded: " + strings.Join(fresh.DigestDegraded, "; ")
						}
						fmt.Fprintf(os.Stderr, "[orchestrator] WARN spine not satisfied for next=%s (a mandatory predecessor's handoff artifact is missing); proceeding fail-open (%s)\n", next, reason)
					}
				}
			}
			o.recordRoutingDecision(ctx, cycle, cs, routingSeq, dec)
		}

		if next == PhaseEnd {
			break
		}

		runner, ok := o.runners[next]
		if !ok {
			// The routing surface (registry order + catalog) can know phases
			// the dispatch surface cannot run (cycle-265: registry-listed
			// `memo` had no .evolve/phases config ⇒ no specrunner; the static
			// order walked into it post-ship and killed a PASSING batch). A
			// non-dispatchable OPTIONAL USER phase is skipped loudly and the
			// walk continues; a missing BUILT-IN or configured-mandatory
			// runner stays fatal — that is a wiring bug, not routing-surface
			// drift (built-ins always have factories; only registry/user
			// phases can be known to routing yet unregistered).
			if next.IsValid() || isConfiguredMandatory(o.cfg, string(next)) {
				return result, fmt.Errorf("%w: no runner registered for phase %s", ErrPhaseInvalid, next)
			}
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s selected but no runner is registered — skipping (register a runner or remove it from the registry order)\n", next)
			current = next
			continue
		}

		phaseStarted := o.now().UTC()
		cs.Phase = string(next)
		cs.PhaseStartedAt = phaseStarted.Format(time.RFC3339)
		cs.ActiveAgent = string(next)
		if err := o.storage.WriteCycleState(ctx, cs); err != nil {
			return result, fmt.Errorf("write cycle-state pre-%s: %w", next, err)
		}

		// CB.1 (concurrency campaign W4): EVERY phase runs with cwd = the cycle
		// worktree — not just the source writers (tdd/build, role-gate-permitted)
		// and audit (issue #9: its verification commands must inspect the
		// builder's pending work). A read-only phase's cwd in the main tree let
		// stray writes and guard misfires land in the live checkout (cycle-280);
		// with the worktree provisioned at cycle start, no phase subprocess
		// touches main at all. cwd is NOT write permission: the write axis
		// (role-gate / tree-diff guard / normalize) still keys off worktreePhase.
		// Empty when provisioning failed — the pre-existing degraded mode.
		phaseWorktree := cs.ActiveWorktree
		// Workstream B: snapshot the main-tree dirty set BEFORE a source-
		// writing phase runs. After it runs we re-snapshot and compare —
		// any newly-dirty MAIN-tree path is a leak that escaped the bridge
		// sandbox (each git worktree is a separate working dir, so its
		// writes don't show up here). The treediff package owns the
		// snapshot/check + SnapshotMissed semantics; the orchestrator just
		// threads it through. Skipped entirely for non-worktree phases.
		var (
			treeGuard      *treediff.Guard
			beforeDirty    []string
			snapshotFailed bool
		)
		if o.gitDirtyPaths != nil {
			treeGuard = treediff.New(o.gitDirtyPaths)
			snap, err := treeGuard.Snapshot(ctx, req.ProjectRoot)
			if err != nil {
				snapshotFailed = true
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff pre-phase snapshot failed for %s: %v (sandbox guard degraded; post-phase leak check skipped)\n", next, err)
			} else {
				beforeDirty = snap
			}
		}
		phaseCtx := ctxSnap
		if next == PhaseRetro {
			phaseCtx = make(map[string]string, len(ctxSnap)+1)
			for k, v := range ctxSnap {
				phaseCtx[k] = v
			}
			phaseCtx["previous_verdict"] = lastVerdict
		}
		phaseReq := PhaseRequest{
			Cycle:         cycle,
			ProjectRoot:   req.ProjectRoot,
			Workspace:     cs.WorkspacePath,
			Worktree:      phaseWorktree,
			RunID:         cs.RunID,
			GoalHash:      req.GoalHash,
			PreviousPhase: string(current),
			Env:           envSnap,
			Context:       phaseCtx,
		}
		// Cycle-122 Fix 3 / ADR-0030: attach the per-phase observer
		// goroutine BEFORE runner.Run and cancel it AFTER. noopObserver
		// (default when WithObserver wasn't used) is byte-identical to
		// the pre-fix cycle. Real implementations spawn a stall detector
		// that watches <workspace>/<agent>-stdout.log and emits stall
		// events to <workspace>/<agent>-observer-events.ndjson.
		// Self-heal (Fix D): a bridge ArtifactTimeout (exit=81) is the
		// recoverable "agent produced no artifact within the wait window" case
		// — a stalled launch where a fresh relaunch usually succeeds. Retry the
		// phase a bounded number of times on THAT sentinel only; every other
		// error (and exhaustion of the budget) aborts the cycle as before. A
		// deterministic timeout (e.g. a misconfigured agent) simply fails again
		// and aborts after the cap — at most one wasted retry. The observer is
		// (re)started per attempt so each launch is watched.
		var resp PhaseResponse
		var err error
		// shipRecovered marks that a ShipError was intercepted and routed to a
		// recovery phase instead of aborting; the post-loop guard then continues
		// the outer loop (skipping verdict/ledger handling for the failed ship).
		shipRecovered := false
		maxAttempts := resolvePhaseMaxAttempts(phaseReq.Env)
		var attemptCount int
		for attempt := 1; ; attempt++ {
			attemptCount = attempt
			obsCancel := o.observer.Start(ctx, string(next), phaseReq)
			resp, err = runner.Run(ctx, phaseReq)
			if obsCancel != nil {
				obsCancel()
			}
			if err == nil && IsVerdict(resp.Verdict) {
				// Self-healing trail: a bridge artifact-wait timeout (exit 81) was
				// reconciled by the runner against a well-formed, gate-passing
				// deliverable, so this phase ships on the agent's own verdict
				// instead of a synthesized FAIL. Record it (mirrors the backfill
				// entry) so the recovery is auditable, never silent.
				if resp.Reconciled {
					if lerr := o.ledger.Append(ctx, LedgerEntry{
						TS:       o.now().UTC().Format(time.RFC3339),
						Cycle:    cycle,
						Role:     string(next),
						Kind:     "reconciled_timeout",
						ExitCode: 81,
					}); lerr != nil {
						fmt.Fprintf(os.Stderr, "[orchestrator] WARN reconciled_timeout ledger append: %v\n", lerr)
					}
				}
				break
			}
			if err != nil {
				if attempt >= maxAttempts || (!errors.Is(err, ErrArtifactTimeout) && !isTransientBridgeError(err)) {
					// Backfill: when exhaustion is specifically due to ErrArtifactTimeout,
					// try to reconstruct the artifact from stdout.clean.txt before aborting.
					// Default-on; disabled only if EVOLVE_BACKFILL_ENABLED is "0" in the
					// per-cycle env SNAPSHOT (ADR-0049 N9: read the snapshot, not live
					// os.Getenv, so a concurrent fleet cycle's env can't flip this cycle's
					// backfill. The snapshot already carries the operator's shell value).
					backfillEnabled := envchain.BoolValue(envSnap["EVOLVE_BACKFILL_ENABLED"], true)
					if attempt >= maxAttempts && errors.Is(err, ErrArtifactTimeout) && backfillEnabled {
						artifactPath := backfillArtifactPath(cs.WorkspacePath, string(next))
						if ok, berr := backfill.TryExtract(cs.WorkspacePath, string(next), artifactPath, 200); berr != nil {
							fmt.Fprintf(os.Stderr, "[orchestrator] WARN backfill %s: %v\n", next, berr)
						} else if ok {
							fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s: ErrArtifactTimeout exhausted; backfilled artifact from stdout.clean.txt; proceeding with WARN verdict\n", next)
							resp = PhaseResponse{Phase: string(next), Verdict: VerdictWARN, ArtifactsDir: cs.WorkspacePath}
							err = nil
							if lerr := o.ledger.Append(ctx, LedgerEntry{
								TS:       o.now().UTC().Format(time.RFC3339),
								Cycle:    cycle,
								Role:     string(next),
								Kind:     "backfill",
								ExitCode: 81,
							}); lerr != nil {
								fmt.Fprintf(os.Stderr, "[orchestrator] WARN backfill ledger append: %v\n", lerr)
							}
							break
						}
					}
					// Optional-phase infra skip (Workstream-D intent on
					// ErrArtifactTimeout; cycle-283): an enrichment phase must not
					// veto completed spine work. When backfill could not reconstruct
					// the artifact, a catalog-Optional, non-floor phase whose
					// exhaustion is infra-shaped degrades to a synthesized WARN and
					// the cycle advances toward audit/ship. The failed attempts stay
					// in failure-learning and the ledger — recovered, never silent.
					if o.optionalInfraSkip(next, err) {
						fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s: optional phase exhausted infra retries (%v); degrading to WARN and advancing (optional_infra_skip)\n", next, err)
						recordFailureLearning(next, fmt.Errorf("phase %s: %w", next, err), attempt)
						if lerr := o.ledger.Append(ctx, LedgerEntry{
							TS:       o.now().UTC().Format(time.RFC3339),
							Cycle:    cycle,
							Role:     string(next),
							Kind:     "optional_infra_skip",
							ExitCode: bridgeExitCode(err),
						}); lerr != nil {
							fmt.Fprintf(os.Stderr, "[orchestrator] WARN optional_infra_skip ledger append: %v\n", lerr)
						}
						resp = PhaseResponse{Phase: string(next), Verdict: VerdictWARN, ArtifactsDir: cs.WorkspacePath}
						err = nil
						break
					}
					// Ship-error recovery seam (Component #7): ship is a pure
					// executor — a structured ShipError is resolved by the advisor's
					// recovery chain (Strategy + CoR), not by aborting the cycle. The
					// resolver records the error, picks the recovery phase
					// (re-audit / retry-ship / debugger), and bounds the depth.
					// Integrity breaches, an illegal edge, or exhausted depth return
					// (_, false) and fall through to the loud abort below.
					if se, ok := AsShipError(err); ok {
						// Preserve the worktree from the exit cleanup while a
						// ship failure is unresolved (ADR-0039 §8 / D10) —
						// cleared when a later ship attempt succeeds.
						preserveWorktree = true
						if rec, recovering := o.recoverFromShipError(ctx, cycle, cs, se, recoveryDepth); recovering {
							ctxSnap["ship_error_code"] = string(se.Code)
							ctxSnap["ship_error_class"] = string(se.Class)
							ctxSnap["ship_error_stage"] = string(se.Stage)
							ctxSnap["ship_error_debug"] = se.DebugString()
							// ADR-0044 C1: the failed ship attempt ran and burned
							// budget — record it before routing to recovery. A
							// later successful ship records its own outcome.
							o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attemptCount,
								fmt.Sprintf("ship error %s: recovering via %s", se.Code, rec)))
							recoveryDepth++
							scheduledNext = rec
							current = PhaseShip // ship ran (and failed); keep forensics accurate
							shipRecovered = true
							break
						}
					}
					phaseErr := fmt.Errorf("phase %s: %w", next, err)
					// ADR-0044 C1: record the dispatch outcome BEFORE the
					// failure-learning retro so the timing record stays
					// chronological (failed phase, then retro). No canonical
					// agent verdict exists on this path → synthesized FAIL.
					o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempt, phaseErr.Error()))
					writePhaseFailureDiag(cs.WorkspacePath, string(next), cycle, err, attempt, o.now)
					// ADR-0044 C3: enforce-only, best-effort — classify the
					// unclassified pane via the LLM tail and promote, so the
					// NEXT occurrence is deterministic. Never alters the abort.
					o.adviseOnUnclassifiedFailure(ctx, cycle, cs.WorkspacePath, req.ProjectRoot, next, err, envSnap)
					recordFailureLearning(next, phaseErr, attempt)
					return result, wrapCycleLevelError(next, phaseErr)
				}
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s attempt %d/%d hit a transient bridge error or timeout; relaunching (self-heal)\n", next, attempt, maxAttempts)
				// Emit structured audit trail for the self-heal retry.
				if lerr := o.ledger.Append(ctx, LedgerEntry{
					TS:       o.now().UTC().Format(time.RFC3339),
					Cycle:    cycle,
					Role:     string(next),
					Kind:     "phase_retry",
					ExitCode: bridgeExitCode(err),
				}); lerr != nil {
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase_retry ledger append: %v\n", lerr)
				}
				executeRetryBackoff(attempt, phaseReq.Env)
				continue
			}
			if err == nil && !IsVerdict(resp.Verdict) {
				if attempt >= maxAttempts {
					ferr := fmt.Errorf("phase %s returned non-canonical verdict %q", next, resp.Verdict)
					// ADR-0044 C1: a non-canonical verdict is never recorded
					// raw and never upgraded — phaseOutcomeFrom synthesizes FAIL.
					o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempt, ferr.Error()))
					writePhaseFailureDiag(cs.WorkspacePath, string(next), cycle, ferr, attempt, o.now)
					recordFailureLearning(next, ferr, attempt)
					return result, wrapCycleLevelError(next, ferr)
				}
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s attempt %d/%d returned non-canonical verdict %q; relaunching\n", next, attempt, maxAttempts, resp.Verdict)
				// Emit structured audit trail for the self-heal retry.
				if lerr := o.ledger.Append(ctx, LedgerEntry{
					TS:       o.now().UTC().Format(time.RFC3339),
					Cycle:    cycle,
					Role:     string(next),
					Kind:     "phase_retry",
					ExitCode: 0,
				}); lerr != nil {
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase_retry ledger append: %v\n", lerr)
				}
				executeRetryBackoff(attempt, phaseReq.Env)
				continue
			}
		}
		if shipRecovered {
			continue // run the recovery phase (scheduledNext) next iteration
		}

		// Workstream E2: per-phase deliverable review gate. Runs ONLY for
		// non-SKIPPED verdicts (a SKIPPED phase produced no deliverable to
		// review) and BEFORE the tree-diff guard + ledger append, so a reject
		// aborts the cycle without recording the phase as a success. The
		// default reviewer is noopReviewer (every phase approved) so opt-out
		// is byte-identical to pre-E2. On reject the correction loop below
		// re-dispatches up to EVOLVE_CONTRACT_CORRECTION_RETRIES times (default
		// 2; 0 = immediate abort, the pre-feature behavior) before aborting.
		if o.reviewer != nil && resp.Verdict != VerdictSKIPPED {
			rin := ReviewInput{
				Phase:       string(next),
				Response:    resp,
				Workspace:   cs.WorkspacePath,
				Worktree:    phaseWorktree,
				ProjectRoot: req.ProjectRoot,
			}
			rr := o.reviewer.Review(ctx, rin)
			// Contract-correction retry: on a deliverable-contract reject,
			// re-dispatch the phase with the violation injected as a
			// "## Correction" directive (bounded by EVOLVE_CONTRACT_CORRECTION_RETRIES,
			// default 2). 0 disables → immediate abort, byte-identical to the
			// pre-feature behavior. This re-runs runner.Run directly (no
			// bridge-timeout retry on corrections — see the design's scope note).
			maxCorrections := resolveContractCorrectionRetries(phaseReq.Env)
			// ADR-0045 I1: a correction re-dispatch is an interaction — every
			// rung of ONE correction decision shares a DecisionID, and each
			// re-dispatch records an outcome resolved by its verdict + the
			// re-review. The I2 ladder's salvage/live-fix rungs will join
			// this same decision when they ship.
			irec := interaction.NewRecorder(cs.WorkspacePath)
			decisionID := ""
			if !rr.Approve && (maxCorrections > 0 || o.contractVerifier != nil) {
				decisionID = fmt.Sprintf("%s-c%d-%d", next, cycle, o.now().UnixNano())
			}
			// ADR-0045 I2: graduated correction ladder. The DECISION is the
			// pure interaction.NextCorrection CoR (salvage → live_fix →
			// redispatch, cheapest first); EXECUTION is stage-gated here.
			// Salvage gets budget only when a breaker-neutral verifier is
			// wired. Rung 2 (live_fix) is decision-complete but
			// execution-dormant at v1: the orchestrator does not yet request
			// named sessions, so NamedREPL is hard-false until the session
			// request + reaper plumbing lands (the C1→C3 deferred-unification
			// precedent; see interaction/correction.go).
			rungBudget := map[string]int{
				interaction.RungSalvage:    1,
				interaction.RungLiveFix:    1,
				interaction.RungRedispatch: maxCorrections,
			}
			if o.contractVerifier == nil {
				rungBudget[interaction.RungSalvage] = 0
			}
			corr := 0
			salvagedFromInvalid := "" // found-but-invalid origin → rung-3 kernel evidence
			for !rr.Approve {
				act := interaction.NextCorrection(interaction.CorrectionInput{
					Phase:      string(next),
					Workspace:  cs.WorkspacePath,
					Worktree:   phaseWorktree,
					Violation:  rr.Reason,
					NamedREPL:  false, // v1: no named-session request plumbing yet
					Busy:       false,
					DecisionID: decisionID,
					RungBudget: rungBudget,
				})
				if act.Rung == "" {
					break // ladder exhausted → abort below, exactly as today
				}
				rungBudget[act.Rung]-- // every iteration spends budget: the loop is finite
				if act.Rung == interaction.RungSalvage {
					salvEv := interaction.Event{
						Kind:       interaction.KindSalvage,
						Phase:      string(next),
						Cycle:      cycle,
						Trigger:    "contract_reject",
						Rung:       interaction.RungSalvage,
						DecisionID: decisionID,
						Payload:    rr.Reason,
					}
					if o.cfg.PhaseRecovery != config.StageEnforce {
						fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: would-salvage misplaced deliverable (%s; EVOLVE_PHASE_RECOVERY=%s)\n", next, act.Reason, o.cfg.PhaseRecovery)
						irec.Record(interaction.Outcome{Event: salvEv, Result: interaction.ResultWouldAct})
						continue
					}
					salvStart := o.now()
					sr := o.salvageDeliverable(ctx, rin)
					switch {
					case sr.Relocated && sr.Verified:
						// Never-upgrades-verdict: the relocated artifact faces
						// the SAME gate — the breaker-touching FINAL outcome.
						rr = o.reviewer.Review(ctx, rin)
						res := interaction.ResultRejectedAgain
						if rr.Approve {
							res = interaction.ResultAccepted
						}
						fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: salvaged %s → contracted path (gate approve=%v)\n", next, sr.From, rr.Approve)
						irec.Record(interaction.Outcome{Event: salvEv, Result: res, LatencyMS: o.now().Sub(salvStart).Milliseconds()})
					case sr.Relocated:
						salvagedFromInvalid = sr.From
						fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: salvage relocated %s but the destination failed verification — falling through\n", next, sr.From)
						irec.Record(interaction.Outcome{Event: salvEv, Result: interaction.ResultFoundButInvalid, LatencyMS: o.now().Sub(salvStart).Milliseconds()})
					default:
						fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: nothing to salvage (%s)\n", next, sr.Reason)
						irec.Record(interaction.Outcome{Event: salvEv, Result: interaction.ResultNotFound, LatencyMS: o.now().Sub(salvStart).Milliseconds()})
					}
					continue
				}
				if act.Rung == interaction.RungLiveFix {
					// Unreachable at v1 (NamedREPL hard-false). Termination
					// invariant for when the named-session plumbing lands:
					// the decrement above already spent this rung's budget,
					// and live_fix never mutates rr — so the `continue` is
					// safe under `for !rr.Approve` and the ladder stays
					// finite (every iteration spends budget or breaks).
					continue
				}
				corr++
				fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: contract violation (correction %d/%d) — re-dispatching with correction: %s\n",
					next, corr, maxCorrections, rr.Reason)
				if lerr := o.ledger.Append(ctx, LedgerEntry{
					TS:       o.now().UTC().Format(time.RFC3339),
					Cycle:    cycle,
					Role:     string(next),
					Kind:     "contract_correction",
					ExitCode: 0,
				}); lerr != nil {
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN contract_correction ledger append: %v\n", lerr)
				}
				// Payload carries the violation that TRIGGERED this dispatch;
				// a rejected_again outcome's NEW violation appears as the next
				// iteration's payload (trigger semantics, not result semantics).
				corrEv := interaction.Event{
					Kind:       interaction.KindCorrectionRedispatch,
					Phase:      string(next),
					Cycle:      cycle,
					Trigger:    "contract_reject",
					Rung:       "redispatch",
					DecisionID: decisionID,
					Payload:    rr.Reason,
				}
				corrStart := o.now()
				recordCorrection := func(res string) {
					irec.Record(interaction.Outcome{
						Event:     corrEv,
						Result:    res,
						LatencyMS: o.now().Sub(corrStart).Milliseconds(),
						CostUSD:   resp.CostUSD,
					})
				}
				directive := composeCorrection(rr.Reason)
				if o.cfg.PhaseRecovery == config.StageEnforce {
					// Evidence-enriched re-dispatch (I2 rung 3): kernel-verified
					// facts only — never agent self-assessment. Shadow keeps
					// today's directive byte-identical.
					if digest := kernelEvidenceDigest(phaseWorktree, salvagedFromInvalid); digest != "" {
						directive += "\n\n" + digest
					}
					// Consume-once: the found-but-invalid note describes what
					// rung 1 discovered for the IMMEDIATE next dispatch; after
					// that agent has rewritten the artifact, repeating the old
					// path would be stale, not kernel-verified.
					salvagedFromInvalid = ""
				}
				phaseReq.CorrectionDirective = directive
				obsCancel := o.observer.Start(ctx, string(next), phaseReq)
				resp, err = runner.Run(ctx, phaseReq)
				if obsCancel != nil {
					obsCancel()
				}
				if err != nil {
					recordCorrection(interaction.ResultDispatchFailed)
					phaseErr := fmt.Errorf("phase %q correction %d dispatch failed: %w", next, corr, err)
					o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attemptCount, phaseErr.Error()))
					recordFailureLearning(next, phaseErr, corr)
					return result, wrapCycleLevelError(next, phaseErr)
				}
				// A correction re-dispatch must produce a canonical verdict to be
				// evaluable, same invariant the outer attempt loop enforces before
				// breaking. Corrections deliberately skip the bridge-timeout retry
				// ladder (scope note), so a non-canonical result here aborts rather
				// than retrying.
				if !IsVerdict(resp.Verdict) {
					recordCorrection(interaction.ResultNonCanonicalVerdict)
					phaseErr := fmt.Errorf("phase %q correction %d produced a non-canonical verdict %q", next, corr, resp.Verdict)
					o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attemptCount, phaseErr.Error()))
					recordFailureLearning(next, phaseErr, corr)
					return result, wrapCycleLevelError(next, phaseErr)
				}
				// rin.Response is refreshed for reviewer consistency; the deliverable
				// reviewer reads the filesystem (workspace/worktree), not this field.
				rin.Response = resp
				rr = o.reviewer.Review(ctx, rin)
				if rr.Approve {
					recordCorrection(interaction.ResultAccepted)
				} else {
					recordCorrection(interaction.ResultRejectedAgain)
				}
			}
			// Defensive: phaseReq is fresh per phase iteration, but never let the
			// directive outlive the loop.
			phaseReq.CorrectionDirective = ""
			if !rr.Approve {
				if maxCorrections == 0 {
					// Byte-identical to the pre-feature abort message.
					phaseErr := fmt.Errorf("review gate: phase %q deliverable rejected: %s", next, rr.Reason)
					// ADR-0044 C1: the phase ran and produced its own verdict;
					// the reject is recorded as the abort reason, not a rewrite.
					o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attemptCount, phaseErr.Error()))
					recordFailureLearning(next, phaseErr, 1)
					return result, wrapCycleLevelError(next, phaseErr)
				}
				phaseErr := fmt.Errorf("review gate: phase %q deliverable rejected after %d correction(s): %s", next, maxCorrections, rr.Reason)
				o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attemptCount, phaseErr.Error()))
				recordFailureLearning(next, phaseErr, maxCorrections)
				return result, wrapCycleLevelError(next, phaseErr)
			}
		}

		if next == PhaseShip && resp.Verdict == VerdictPASS {
			// Ship landed AND survived the deliverable review gate above — the
			// worktree is merged, normal exit cleanup applies. Deliberately
			// AFTER the review gate: a review-rejected ship abort must still
			// preserve the worktree for triage (ADR-0039 §8 / D10).
			preserveWorktree = false
		}

		// Workstream B: post-phase tree-diff check. Runs BEFORE the ledger
		// append so a leak aborts the cycle without recording the phase as a
		// success. Snapshot failures (pre OR post) degrade silently — the
		// guard is belt-and-suspenders to the OS sandbox, so a transient git
		// read error must never cause a false abort.
		// Cycle-160 fix (Option A): a non-Claude builder (agy/codex in tmux) is
		// not bound by the Claude-only role-gate, and the OS sandbox is off on
		// nested-macOS, so it can write build output to the MAIN tree instead of
		// its worktree. Relocate any such leak into the worktree (staged, so audit
		// sees it) and restore main BEFORE the tree-diff guard runs. Runs
		// unconditionally after build (no-op when clean) because the guard's
		// `git diff --name-only HEAD` baseline is tracked-only and misses
		// pure-untracked leaks. On recovery FAILURE we abort explicitly — the
		// tree-diff guard only backstops tracked leaks, so a failed recovery
		// of an untracked leak must not slip past into audit.
		if WorktreePhase(next) && cs.ActiveWorktree != "" {
			if !recoverBuildLeak(ctx, req.ProjectRoot, cs.ActiveWorktree, mainDirtyBaseline) {
				phaseErr := fmt.Errorf("phase %s: worktree-leak recovery failed (main tree left unsafe for audit)", next)
				o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attemptCount, phaseErr.Error()))
				recordFailureLearning(next, phaseErr, 1)
				return result, phaseErr
			}
		}

		if treeGuard != nil && !snapshotFailed {
			res := treeGuard.Check(ctx, req.ProjectRoot, beforeDirty)
			if res.SnapshotMissed {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff post-phase snapshot failed for %s (sandbox guard degraded; not aborting)\n", next)
			} else if !res.OK() {
				// Attempt phase-agnostic binary churn discard for build artifacts
				var relBin string
				if execPath, err := os.Executable(); err == nil {
					if rel, err := filepath.Rel(req.ProjectRoot, execPath); err == nil && !strings.HasPrefix(rel, "..") {
						relBin = filepath.ToSlash(rel)
					}
				}

				// Always discard "go/evolve" and relBin (if set)
				_ = discardMainLeak(ctx, req.ProjectRoot, "go/evolve")
				if relBin != "" && relBin != "go/evolve" {
					if isGitignored(ctx, req.ProjectRoot, relBin) {
						fmt.Fprintf(os.Stderr, "[orchestrator] WARN: relBin path %q is gitignored; skipping discardMainLeak to prevent checkout error\n", relBin)
					} else {
						_ = discardMainLeak(ctx, req.ProjectRoot, relBin)
					}
				}

				// Re-snapshot and check again
				res2 := treeGuard.Check(ctx, req.ProjectRoot, beforeDirty)
				if res2.OK() {
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff: discarded binary rebuild churn in phase %s; continuing\n", next)
				} else {
					// Filter the leaked set through isLegitimateMainTreePath for EVERY
					// phase — the same classification recoverBuildLeak applies (R9: one
					// shared vocabulary). Non-worktree phases need it for their
					// .evolve/ workspace writes (R7); worktree phases need it because
					// orchestrator-side gates write their own untracked runtime state
					// (.evolve/contract-gate-breaker.json) into the main tree mid-phase
					// — recovery skips those by design, so a strict guard here turned
					// every contract-gate trip into a false cycle abort (the cycle-274
					// salvage CI regression). PLUS a guard-only second classifier,
					// isScoutEvalMaterialization: scout writes its selected evals to the
					// main tree by contract (materialization.go), which recoverBuildLeak
					// never sees (scout is not a WorktreePhase) so it lives only here
					// (soak-#6 cycle 318→319). Real escapes stay armed: source files and
					// non-scout/non-eval deliverable paths classify as leaks, and
					// porcelainDirtySet emits both rename sides so a deliverable renamed
					// to a .evolve/evals/ look-alike still aborts via its source path.
					leaked := res2.Leaked
					var realLeaks []string
					for _, p := range leaked {
						if isLegitimateMainTreePath(p) || isScoutEvalMaterialization(next, p) {
							continue
						}
						realLeaks = append(realLeaks, p)
					}
					if len(realLeaks) == 0 {
						fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff: phase %s wrote only legitimate main-tree paths (.evolve/ workspace); continuing\n", next)
						leaked = nil
					} else {
						leaked = realLeaks
					}
					if len(leaked) > 0 {
						phaseErr := fmt.Errorf("tree-diff guard: phase %q wrote to the main tree outside its worktree %q — leaked paths: %v",
							string(next), phaseWorktree, leaked)
						// ADR-0044 C1 — THE cycle-262 path: the build ran, PASSed,
						// and burned tokens before the guard caught its main-tree
						// leak. The abort is correct; erasing the outcome was not.
						o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attemptCount, phaseErr.Error()))
						recordFailureLearning(next, phaseErr, 1)
						// After abort, check if go/bin/evolve is absent
						evolveBinPath := filepath.Join(req.ProjectRoot, "go/bin/evolve")
						if _, err := os.Stat(evolveBinPath); os.IsNotExist(err) {
							fmt.Fprintf(os.Stderr, "[orchestrator] ABNORMAL: go/bin/evolve absent after cycle abort — trust-kernel guards degraded\n")
						}
						return result, phaseErr
					}
				}
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
			// ADR-0044 C1: the phase completed; a persistence failure must
			// not erase its outcome from the timing/usage record.
			o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attemptCount, lerr.Error()))
			return result, lerr
		}

		o.emitPhaseBindings(ctx, cycle, req.ProjectRoot, cs, next, resp.Verdict)

		// Cycle-156 fix (Option C): a committing builder (e.g. agy/Gemini
		// following evolve-builder.md:235) leaves its work in a worktree
		// commit, but audit + binding inspect `git diff HEAD` (empty after a
		// commit). Soft-reset the build's commits to the cycle base so the
		// work is pending again before audit runs (next iteration). No-op for
		// non-committing builders. See the cycle-156 incident doc.
		o.normalizeBuildWorktree(ctx, next, cs)

		cs.CompletedPhases = append(cs.CompletedPhases, string(next))
		if err := o.storage.WriteCycleState(ctx, cs); err != nil {
			werr := fmt.Errorf("write cycle-state post-%s: %w", next, err)
			o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attemptCount, werr.Error()))
			return result, werr
		}

		if PhaseBoundaryCheckpointer != nil {
			if err := PhaseBoundaryCheckpointer(cs, req.ProjectRoot, o.now()); err != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase boundary checkpoint failed: %v\n", err)
			}
		}

		result.FinalVerdict = resp.Verdict
		o.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attemptCount, ""))
		current = next
		lastVerdict = resp.Verdict

		// Retro is the one phase whose successor isn't verdict-driven;
		// the failure-adapter consults cycle history (state.FailedAt) and
		// the retro verdict to pick {ship | tdd | end}. Set scheduledNext
		// so the next loop iteration runs the chosen phase.
		if current == PhaseRetro {
			var branch Phase
			var extraEnv map[string]string
			var reason string
			if o.cfg.Stage >= config.StageAdvisory {
				// Failure floor Phase 3: the failure branch is advisor-
				// decidable (clamped) and leaves a routing-decision artifact.
				routingSeq++
				branch, extraEnv, reason = o.decideAfterRetroRouted(ctx, cycle, cs, routingSeq, resp.Verdict, state.FailedAt, router.RouteInput{
					Cfg:            o.cfg,
					Completed:      cs.CompletedPhases,
					Strict:         envchain.BoolValue(envSnap["EVOLVE_STRICT_AUDIT"], false),
					Workspace:      cs.WorkspacePath,
					ProjectRoot:    req.ProjectRoot,
					Cycle:          cycle,
					Env:            envSnap,
					Plan:           clampedPlan,
					IntentRequired: cs.IntentRequired,
					PSMASEnabled:   envchain.BoolValue(envSnap["EVOLVE_PSMAS_SKIP"], false),
				})
			} else {
				branch, extraEnv, reason = o.decideAfterRetro(resp.Verdict, state.FailedAt)
			}
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

		// The debugger phase is decision-driven (RESHIP / RERUN_PHASE / BLOCK),
		// not verdict-driven — mirror the retro branch. The debugger runner
		// surfaces its decision on PhaseResponse.Signals; decideAfterDebugger
		// maps it to the next phase, which the next iteration runs via
		// scheduledNext (bypassing the routing override, like retro).
		if current == PhaseDebugger {
			branch := o.decideAfterDebugger(resp)
			o.recordDebuggerDecision(ctx, cycle, cs, resp)
			if branch == PhaseEnd {
				break
			}
			if !o.sm.CanTransition(PhaseDebugger, branch) {
				return result, fmt.Errorf("debugger→%s not allowed by state machine", branch)
			}
			scheduledNext = branch
		}
	}

	// Post-loop finalization (verdict reclassification, silent-no-ship warn,
	// throughput, worktree-preserve decision, state persist) → finalizeCycle.
	// preserveWorktree is threaded back so the exit defer (registered above)
	// observes it; cycleCompletedNormally is set only on a clean persist.
	preserve, ferr := o.finalizeCycle(ctx, cs, cycle, preCycleHEAD, &result, &state)
	if preserve {
		preserveWorktree = true
	}
	if ferr != nil {
		return result, ferr
	}
	cycleCompletedNormally = true
	return result, nil
}
