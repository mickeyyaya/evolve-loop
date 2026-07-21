package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/directives"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/research"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/verdictcache"
)

// PhaseBoundaryCheckpointer is a package-level hook to write a checkpoint block
// at phase boundaries, set by the checkpoint package to avoid circular imports.
var PhaseBoundaryCheckpointer func(cs CycleState, projectRoot string, now time.Time) error

// QuotaBoundaryCheckpointer is a package-level hook to write a quota-likely
// checkpoint block when the dispatch seam detects all-families exit=85
// exhaustion (cycle-656). Set by the checkpoint package to avoid circular
// imports; it preserves completed phases + the worktree so `evolve loop
// --resume` re-enters at the deferred phase after the quota resets.
var QuotaBoundaryCheckpointer func(cs CycleState, projectRoot string, now time.Time) error

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
// intent documented on ErrArtifactTimeout; cycle-283). Four conditions, all
// required: the error is INFRA-shaped (artifact timeout / transient bridge —
// never integrity or logic failures), the phase is NOT configured-mandatory,
// the phase is catalog-Optional, and the phase sits outside the resolved ship
// floor — so the skip can never weaken `ship ⇒ build ∧ audit ∧ tdd`. The
// mandatory guard is generic and config-driven (the orchestrator reads
// cfg.Mandatory, not a hardcoded phase name): it subsumes the former ship
// special-case — ship is a mandatory anchor — while protecting any mandatory
// phase mis-marked Optional (the floor loop alone would miss ship, which is
// not in the floor set). Phase-agnostic flow per ADR-0035/0038.
func (o *Orchestrator) optionalInfraSkip(p Phase, err error) bool {
	if !errors.Is(err, ErrArtifactTimeout) && !isTransientBridgeError(err) {
		return false
	}
	if isConfiguredMandatory(o.cfg, string(p)) {
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

// postShipObserverSkip classifies a POST-ship best-effort observer failure as
// non-fatal (cycle-574, inbox memo-phase-tier-envelope). A RoleControl observer
// phase that runs after a healthy ship (memo, post-ship-monitor) must never turn
// an already-shipped cycle abnormal: its failure degrades to a WARN diagnostic
// and the cycle keeps its shipped/PASS outcome. This is a sibling to
// optionalInfraSkip — same floor/mandatory guards, but keyed on ship-having-
// -landed rather than an infra-shaped error, so it also covers a memo policy or
// logic error (the exact tier-envelope shape that reddened healthy cycles).
// True iff ALL hold: (1) ship already recorded PASS this cycle (shipped) — a
// failure BEFORE ship is never swallowed; (2) p is a catalog-Optional RoleControl
// observer and NOT ship itself; (3) p is not configured-mandatory and sits
// outside the resolved ship floor — the skip can never weaken the integrity floor.
func (o *Orchestrator) postShipObserverSkip(p Phase, shipped bool) bool {
	if !shipped {
		return false
	}
	if p == PhaseShip {
		return false
	}
	if isConfiguredMandatory(o.cfg, string(p)) {
		return false
	}
	spec, ok := o.catalog.Get(string(p))
	if !ok || !spec.Optional {
		return false
	}
	if spec.RoleOrDefault() != phasespec.RoleControl {
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

// specFor resolves a phase's descriptor, canonicalizing the name first
// (PhaseRetro→"retrospective") so the lookup cannot silently miss on the
// core↔router skew. The registry is the SSOT and wins; on a registry miss it
// falls to the builtinControlSpec seam (ADR-0058 §5) for control phases that have
// no registry home (debugger). A total miss yields (_, false), which keeps Next
// on its byte-identical literal path. It is the StateMachine's window onto
// config-driven transition resolution (ADR-0058).
func (o *Orchestrator) specFor(p Phase) (phasespec.PhaseSpec, bool) {
	if spec, ok := o.catalog.Get(canonicalCatalogName(p)); ok {
		return spec, true // registry SSOT wins over the control seam
	}
	return builtinControlSpec(p)
}

// phaseArchetype resolves a phase's composition class (plan/build/evaluate/
// control) from the existing phasespec taxonomy — the registry spec's explicit
// archetype when present, else name inference. The single source the latency
// roll-up buckets by; see recordPhaseOutcome (ADR-0044 C1 chokepoint).
func (o *Orchestrator) phaseArchetype(phase string) string {
	if spec, ok := o.specFor(Phase(phase)); ok {
		return string(spec.RoleOrDefault())
	}
	return string(phasespec.PhaseSpec{Name: phase}.RoleOrDefault())
}

// builtinControlSpec is the control-phase metadata seam (ADR-0058 §5): the one
// place Go data describes a phase, justified because the control phases
// (debugger; start/end) are registered as runners in cmd_cycle.go and have no
// registry `phases[]` home. It supplies ONLY branch metadata — debugger's
// signal-driven successor — never an OnPass/OnFail edge, so Next stays literal
// for control phases. A registry entry of the same name overrides it (specFor
// precedence). Returns (_, false) for any phase the seam does not describe.
func builtinControlSpec(p Phase) (phasespec.PhaseSpec, bool) {
	if p == PhaseDebugger {
		return phasespec.PhaseSpec{
			Name:              string(PhaseDebugger),
			BranchingStrategy: phasespec.BranchingSignal,
			// PA-DDK DDK-6: the debugger's recovery targets are config-declared in
			// the control seam (it has no registry home), mirroring the literal —
			// RESHIP→ship, RERUN_PHASE→audit (the default upstream), BLOCK→end.
			Recovery: &phasespec.RecoveryMap{Targets: map[string]string{
				"RESHIP":      string(PhaseShip),
				"RERUN_PHASE": string(PhaseAudit),
				"BLOCK":       string(PhaseEnd),
			}},
		}, true
	}
	return phasespec.PhaseSpec{}, false
}

// successorStrategy resolves how phase p's successor is chosen — the
// branching_strategy declared on its descriptor (ADR-0058). On a catalog miss,
// or an entry that omits the field, it degrades to the literal phase-identity
// default (literalSuccessorStrategy), keeping the flow byte-identical when the
// catalog is unset. "Config selects, code constrains": this only routes the
// orchestrator among branches the state machine already deems legal — it never
// invents an edge.
func (o *Orchestrator) successorStrategy(p Phase) string {
	if spec, ok := o.specFor(p); ok && spec.BranchingStrategy != "" {
		return spec.BranchingStrategy
	}
	return literalSuccessorStrategy(p)
}

// literalSuccessorStrategy is the unconfigured backstop: the successor-selection
// strategy each phase used before ADR-0058 made it config-driven. Two phases are
// not verdict-driven: retrospective is history-driven (the failure-adapter
// consults cycle history) and debugger is signal-driven (its decision signal
// picks the successor). Every other phase is verdict-driven (the empty default).
func literalSuccessorStrategy(p Phase) string {
	switch p {
	case PhaseRetro:
		return phasespec.BranchingHistory
	case PhaseDebugger:
		return phasespec.BranchingSignal
	}
	return ""
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
	// DisableWorkspaceGuard skips the pre-cycle workspace archive that
	// evicts stale phase artifacts from a prior interrupted cycle. Used by
	// tests that pre-seed workspace files to simulate phase state; operators
	// may set it via the env snapshot read in cmd_cycle.go.
	DisableWorkspaceGuard bool
	// BypassPolicy skips the policy.json pin enforcement for every phase in
	// this cycle. Used for testing and escape-hatch operator overrides.
	// Threaded to PhaseRequest.BypassPolicy at dispatch.
	BypassPolicy bool
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

	// gitMutationLock serializes the shared-main-repo dossier closeout commit
	// against concurrent fleet lanes on the integrator's .evolve/ship.lock, so a
	// lane's `dossier: cycle-N closeout` commit never races a sibling's ship
	// commit on .git/index.lock (fleet-ship-git-index-lock-serialization). nil ⇒
	// fail-open (the commit's bounded index.lock retry is the backstop). Defaulted
	// to defaultGitMutationLock; a test seam swaps in a deterministic spy.
	gitMutationLock gitMutationLocker

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

	// modelCatalogLookup is the optional live model-catalog resolvability check
	// (WithModelCatalogLookup) injected into router.ClampPlanModelRouting. Nil
	// (default) ⇒ the catalog-resolvability gate is skipped (guardrail
	// validation still runs); the composition root wires modelcatalog.Catalog.Lookup.
	modelCatalogLookup func(cli, tier string) (string, bool)

	// directivesProvider optionally returns the runtime operator-directives snapshot
	// for a cycle (WithDirectivesProvider). The injected closure owns ALL config —
	// home/lane/path resolution — so core stays config- and environment-agnostic; it
	// is fail-open (returns a possibly-empty Set, never errors). nil ⇒ no directives.
	directivesProvider func(ctx context.Context, cycle int) directives.Set

	// compositionSnapshot, compositionGateRunner, and compositionVerdictWriter
	// wire the RUNG 0 trivial-rebase composition-verdict fast path
	// (WithCompositionSnapshot / WithCompositionGateRunner /
	// WithCompositionVerdictWriter) into recoverFromShipError's clean
	// fleet-rebase branch. All three nil (default) ⇒ the fast path never
	// fires ⇒ recovery behaves exactly as it does today (byte-identical).
	// The composition root binds the writer closure to the real
	// ledger.WriteCompositionVerdict; core stays adapter-agnostic (ledger
	// already imports core, so a direct import would cycle).
	compositionSnapshot      func(ctx context.Context, worktree string) (CompositionAuditSnapshot, error)
	compositionGateRunner    func(ctx context.Context, worktree string) map[string]string
	compositionVerdictWriter func(ledgerPath string, in CompositionVerdictInput) error

	// scopedMergeReviewer wires the merge ladder's RUNG 2 scoped merge review
	// (WithScopedMergeReviewer) into recoverFromShipError, between the RUNG 0
	// carry-forward miss and the RUNG 3 full re-audit. Nil (default) ⇒ RUNG 2
	// stays dark ⇒ recovery behaves exactly as it does today (no regression).
	scopedMergeReviewer ScopedMergeReviewer

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

	// safetyViolations is the ValidateSafetyInvariants result computed once at
	// construction over the wired SM + cfg + catalog (PA-DDK DDK-5). Non-empty ⇒
	// the loaded transition config could ship without the floor; RunCycle /
	// RunCycleFromPhase fail closed with ErrUnsafeConfig before any phase runs.
	// nil (the bare/empty config) ⇒ no violations ⇒ unchanged behavior.
	safetyViolations []string

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

	// retryConfig is resolved once from policy.json at the composition root.
	retryConfig policy.RetryConfig

	// workflowConfig is resolved once from policy.json at the composition root.
	workflowConfig policy.WorkflowConfig

	// chronicle is the resolved chronicle policy (digest stage/caps), resolved
	// once from policy.json at the composition root (chronicle S3).
	chronicle policy.ChronicleConfig

	// failurePolicy is the resolved system-failure DECISION policy (ADR-0072):
	// the category→action map + Go-enforced floor. Resolved once from
	// policy.json at the composition root; absent ⇒ compiled defaults.
	failurePolicy policy.SystemFailurePolicy

	// maxPhaseIterations bounds RunCycle's dispatch loop (the transition-table
	// cycle guard). 0 ⇒ defaultMaxPhaseIterations. Injected via
	// WithMaxPhaseIterations; tests set it low to exercise the C1
	// chokepoint-escape guard deterministically.
	maxPhaseIterations int

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

// WithRetryConfig injects the resolved retry policy.
func WithRetryConfig(cfg policy.RetryConfig) Option {
	return func(o *Orchestrator) { o.retryConfig = cfg }
}

// WithWorkflowConfig injects the resolved workflow policy.
func WithWorkflowConfig(cfg policy.WorkflowConfig) Option {
	return func(o *Orchestrator) { o.workflowConfig = cfg }
}

// WithChronicleConfig injects the resolved chronicle policy (chronicle S3:
// recent-outcomes digest stage + caps). The zero-option default is the
// compiled default (digest=shadow).
func WithChronicleConfig(cfg policy.ChronicleConfig) Option {
	return func(o *Orchestrator) { o.chronicle = cfg }
}

// WithFailurePolicy injects the resolved system-failure decision policy
// (ADR-0072). The zero-option default is the compiled DefaultSystemFailurePolicy.
func WithFailurePolicy(fp policy.SystemFailurePolicy) Option {
	return func(o *Orchestrator) { o.failurePolicy = fp }
}

// WithMaxPhaseIterations overrides the dispatch-loop iteration bound (the
// transition-table cycle guard). n<=0 is ignored so the defaultMaxPhaseIterations
// safety oracle stands; tests set it low to drive RunCycle into the C1
// chokepoint-escape path deterministically.
func WithMaxPhaseIterations(n int) Option {
	return func(o *Orchestrator) {
		if n > 0 {
			o.maxPhaseIterations = n
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

// WithWorktreeBase injects the operator worktree-base override, resolved once
// from policy.json (worktree.base) at the composition root. Empty ⇒ no override
// (the gitWorktree default <root>/.evolve/worktrees stands). Replaces the former
// EVOLVE_WORKTREE_BASE env read (flag-reduction, ADR-0064). Mutually exclusive
// with WithWorktreeProvisioner (both set o.worktree); production uses only this
// one, tests use only the fake — never both.
func WithWorktreeBase(base string) Option {
	return func(o *Orchestrator) {
		if base != "" {
			o.worktree = gitWorktree{baseOverride: base}
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

// WithModelCatalogLookup injects the live model-catalog resolvability check
// (cycle-440 MR4a) consulted by router.ClampPlanModelRouting: (cli,tier)→
// (model,ok). Nil (default) skips the catalog-resolvability gate — the plan's
// guardrail validation (allowed_clis/model_tier_envelope) still applies. The
// composition root wires modelcatalog.Catalog.Lookup so core stays a leaf and
// never imports modelcatalog directly (dependency inversion, mirroring
// catalogRefresh above).
func WithModelCatalogLookup(fn func(cli, tier string) (string, bool)) Option {
	return func(o *Orchestrator) { o.modelCatalogLookup = fn }
}

// WithDirectivesProvider injects the runtime operator-directives provider. The
// closure (wired at the composition root) owns ALL config — home/lane/path
// resolution — and re-reads the directive files each cycle so live operator edits
// propagate at the next cycle boundary; the orchestrator snapshots the result once
// per cycle, stamps its version into the ledger, and threads it to every phase. A
// nil fn leaves directives off (byte-identical dispatch).
func WithDirectivesProvider(fn func(ctx context.Context, cycle int) directives.Set) Option {
	return func(o *Orchestrator) { o.directivesProvider = fn }
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
// HasRunner reports whether a PhaseRunner is registered for p. It is the
// composition-root's read seam: a phase the router can nominate but that has
// no runner is silently skipped by cyclerun_dispatch's missing-runner escape
// hatch (the cycle-563 memo-dispatch bug), so tests assert on this to prove the
// routing→dispatch handoff is actually wired, not just that Route() names it.
func (o *Orchestrator) HasRunner(p Phase) bool {
	_, ok := o.runners[p]
	return ok
}

func NewOrchestrator(storage Storage, ledger Ledger, runners map[Phase]PhaseRunner, opts ...Option) *Orchestrator {
	o := &Orchestrator{
		storage:         storage,
		ledger:          ledger,
		runners:         runners,
		sm:              NewStateMachine(),
		now:             time.Now,
		gitHEAD:         defaultGitHEAD,
		gitMutationLock: defaultGitMutationLock,
		gitDirtyPaths:   defaultGitDirtyPaths,
		worktree:        gitWorktree{},
		strategy:        router.StaticPreset{},
		retryConfig:     policy.Policy{}.RetryConfig(),
		workflowConfig:  policy.Policy{}.WorkflowConfig(),
		chronicle:       policy.Policy{}.ChronicleConfig(),
		failurePolicy:   policy.DefaultSystemFailurePolicy(),
		reviewer:        noopReviewer{}, // WS-E2: byte-identical default until WithReviewer is used
		observer:        noopObserver{}, // cycle-122 Fix 3 / ADR-0030: byte-identical default until WithObserver is used
	}
	for _, opt := range opts {
		opt(o)
	}
	// ADR-0058: hand the state machine its config-driven verdict-branch
	// resolution now that the catalog (hence specFor) is settled by options.
	// Without a catalog, specFor misses and Next stays on the literal table
	// (byte-identical). PA-DDK DDK-3: the linear spine is now config-declared
	// (cfg.SpineOrder); an empty order leaves the SM on the canonical literal.
	o.sm.WithCatalog(o.specFor).WithSpine(spinePhasesFrom(o.cfg.SpineOrder)).
		WithLegalGraph(legalGraphFrom(o.cfg.LegalSuccessors))
	// PA-DDK DDK-5 (ADR-0060 §1a): with the legality graph + gates + verdict
	// branches all config-driven, the floor's only structural guarantee is the
	// phase-agnostic validator. Compute its verdict once over the fully-wired SM;
	// RunCycle/RunCycleFromPhase fail closed if it found a floor hole. A bare/empty
	// config yields no violations, so default orchestrators are unaffected.
	o.safetyViolations = ValidateSafetyInvariants(o.sm, o.cfg, o.catalog)
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
	return envchain.BoolValue(env[ipcenv.FleetKey], false)
}

// RunCycle drives one cycle from PhaseStart to PhaseEnd, returning a
// summary of what ran. The lock is acquired up front (except in fleet mode,
// see fleetMode) and released on every exit path. State is updated
// incrementally so a crash leaves an inspectable trail in .evolve/.
// ensureSafeConfig fails closed when the loaded transition config violates a
// safety invariant (PA-DDK DDK-5). It is the run-time half of the relocated
// trust anchor: the composition root computes the violations at construction;
// every cycle-run entry refuses to proceed if the floor could be bypassed, so an
// unsafe registry edit can never run a single phase.
func (o *Orchestrator) ensureSafeConfig() error {
	if len(o.safetyViolations) > 0 {
		return fmt.Errorf("%w: %s", ErrUnsafeConfig, strings.Join(o.safetyViolations, "; "))
	}
	return nil
}

func (o *Orchestrator) RunCycle(ctx context.Context, req CycleRequest) (CycleResult, error) {
	if err := o.ensureSafeConfig(); err != nil {
		return CycleResult{}, err
	}
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
		retryConfig:       o.retryConfig,
		workflowConfig:    o.workflowConfig,
		// scheduledNext "", routingSeq 0, recoveryDepth 0, preserveWorktree false,
		// cycleCompletedNormally false — zero-valued by construction.
	}
	// Cleanup defer registered FIRST (fires LAST, LIFO) and BEFORE planCycle —
	// byte-identical registration order to the inline version. Reads the LATE
	// field values at exit (R2 late-visibility): the single most important
	// state-promotion of the method-object refactor.
	defer func() { cleanup(cr.preserveWorktree, cr.cycleCompletedNormally) }()
	// Cycle-778: no exit path (abort, chokepoint escape, panic-free error
	// return) may leave the ship-window lease held — siblings would wait out
	// the full TTL. Idempotent; the normal release happens in recordAndBranch.
	defer cr.releaseShipWindow()

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
	cr.directivesSet = plan.directivesSet

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

	// Bounded loop guards against any transition-table cycle bug. The bound is
	// injectable (WithMaxPhaseIterations) but defaults to the safety oracle.
	// Labeled so the extracted sub-methods can signal loop termination
	// (loopBreak → `break OuterLoop`) from inside the switch ladder below —
	// a bare `break` there would exit the switch, not the loop (H15).
	maxIter := o.maxPhaseIterations
	if maxIter <= 0 {
		maxIter = defaultMaxPhaseIterations
	}
OuterLoop:
	for safety := 0; safety < maxIter; safety++ {
		// Static transition + dynamic-routing override + spine-integrity gate +
		// PhaseEnd termination → selectNext.
		next, act, serr := cr.selectNext()
		switch act {
		case loopAbort:
			return cr.result, serr
		case loopBreak:
			cr.reachedPhaseEnd = true
			break OuterLoop
		}

		// PR2b: at ParallelEvaluate=enforce, when `next` begins a run of
		// independent post-build checking phases (archetype "evaluate", audit
		// excluded), dispatch the whole run CONCURRENTLY as one batch and skip
		// the per-phase path. StageOff/Shadow (the default) never enter here, so
		// the sequential dispatch below is byte-identical to pre-PR2b.
		if cr.o.cfg.ParallelEvaluate == config.StageEnforce {
			if batch := cr.evaluateBatchAt(next); len(batch) >= 2 {
				if bact, berr := cr.dispatchEvaluateBatch(batch); bact == loopAbort {
					return cr.result, berr
				}
				continue
			}
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

		// Graduated remediation (2026-07-21): a configured deterministic gate
		// that FAILed gets one bounded builder fix + a same-gate re-run BEFORE
		// the verdict is recorded. Nothing downstream is bypassed — the same
		// gate must pass and audit/EGPS/ship floors run unchanged.
		if act, merr := cr.maybeRemediate(next, &dr); act == loopAbort {
			return cr.result, merr
		}

		// End-of-iteration record + branch (success ledger, bindings, normalize,
		// CompletedPhases persist, checkpoint, outcome record + cursor advance,
		// retro/debugger non-verdict-driven branches) → recordAndBranch.
		switch act, berr := cr.recordAndBranch(next, dr); act {
		case loopAbort:
			return cr.result, berr
		case loopBreak:
			cr.reachedPhaseEnd = true
			break OuterLoop
		}

		// WS2-S0 (ADR-0052): post-scout re-plan hook. Fires once per cycle after
		// scout's handoff has been recorded (recordAndBranch above) and BEFORE the
		// next selectNext — gated on the just-completed phase being scout. Firing
		// here (post-record, pre-select) is what keeps the re-plan from widening the
		// run-set or bypassing the spine gate. No-op until WS2-S3 wires the shadow
		// RePlan behind EVOLVE_ROUTER_REPLAN.
		if next == PhaseScout {
			// Lane-scope reconciliation (cycle-640 pin; supersedes the old
			// hard-abort gate). The scout is asked to echo the pinned goal_hash
			// into its Decision Trace, but that echo is a FRAGILE signal — a
			// deterministic LLM transcription flip false-aborted healthy cycles
			// before triage (cycles 945/947/... — greedy decoding reproduces the
			// same wrong digit, so retries never self-heal). The pinned
			// lane-scope.json goal_hash is authoritative, so a divergence is
			// machine-STAMPED into the report (triage proceeds on a coherent lane)
			// with a WARN, not an abort. Fail-open on missing pin/report/hash.
			normalizeScoutGoalHash(cr.cs.WorkspacePath)
			cr.postScoutReplan()
		}
	}

	// ADR-0044 C1 chokepoint-escape guard: the bounded loop can exit by
	// exhausting its iteration budget (a transition-table cycle) instead of
	// reaching PhaseEnd. That exit recorded no terminal outcome, so
	// cyclehealth.ClassifyOutcome would page the cycle FAILED_UNEXPLAINED — the
	// alarm bucket (the cycle-492 escape). Record an explicit abort so the escape
	// is FAILED_EXPLAINED and diagnosable: it names the phase the cursor stalled
	// on. Runs BEFORE finalizeCycle so the recorded FAIL preserves the worktree
	// for salvage.
	if !cr.reachedPhaseEnd {
		cr.recordChokepointEscape(fmt.Sprintf(
			"transition-table cycle guard: dispatch loop ran %d iterations without reaching PhaseEnd (cursor stalled at phase %q) — a transition cycle prevented termination; ADR-0044 C1 chokepoint escape",
			maxIter, cr.current))
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
	// ADR-0055: emit this completed cycle's closeout dossier to
	// <ProjectRoot>/knowledge-base/cycles/cycle-N.json. Best-effort — the cycle
	// has already finalized, so a closeout-artifact write error must not fail it
	// (presence is enforced separately by `evolve dossier verify` against the
	// policy floor). Goal text comes from Context["goal"]; falls back to the goal
	// hash so the dossier's required Goal is never blank.
	dossierGoal := cr.req.Context["goal"]
	if dossierGoal == "" {
		dossierGoal = cr.req.GoalHash
	}
	if derr := writeCycleDossier(cr.o.gitMutationLock, cr.req.ProjectRoot, cr.cs.WorkspacePath, cr.cycle, dossierGoal, cr.cs.RunID, cr.result.FinalVerdict, cr.result.SkippedPhases); derr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d: closeout dossier not written (non-fatal): %v\n", cr.cycle, derr)
	}
	return cr.result, nil
}
