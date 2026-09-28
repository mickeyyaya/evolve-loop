package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/failurediag"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/failurelearning"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/outcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/directives"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/research"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/verdictcache"
)

// PhaseBoundaryCheckpointer is a package-level hook to write a checkpoint block
// at phase boundaries, set by the checkpoint package to avoid circular imports.
var PhaseBoundaryCheckpointer func(cs CycleState, projectRoot string, now time.Time) error

// QuotaBoundaryCheckpointer is a package-level hook to write a quota-likely
// checkpoint block when the dispatch seam detects all-families quota
// exhaustion. Set by the checkpoint package to avoid circular imports.
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

func optionalSkipDetails(p Phase, err error) (kind, msg string, diags []Diagnostic) {
	if errors.Is(err, ErrAgentDocMissing) {
		msg = fmt.Sprintf("optional phase %s: persona doc missing (%v) — skipping with a warning; provide the phase persona before selecting it again", p, err)
		return "optional_missing_persona_skip", msg, []Diagnostic{{Severity: "warn", Message: msg}}
	}
	msg = fmt.Sprintf("optional phase %s exhausted infra retries (%v) — skipping with a warning", p, err)
	return "optional_infra_skip", msg, []Diagnostic{{Severity: "warn", Message: msg}}
}

func (o *Orchestrator) optionalInfraSkip(p Phase, err error) bool {
	if !IsOptionalSkippableError(err) {
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

// specFor resolves a phase's descriptor, canonicalizing the name first so the
// lookup cannot miss on the core↔router spelling skew. The registry is the
// SSOT and wins over the builtinControlSpec fallback for phases with no
// registry entry.
func (o *Orchestrator) specFor(p Phase) (phasespec.PhaseSpec, bool) {
	if spec, ok := o.catalog.Get(canonicalCatalogName(p)); ok {
		return spec, true
	}
	return builtinControlSpec(p)
}

// phaseArchetype resolves a phase's composition class from the registry
// spec's archetype, falling back to name inference; recordPhaseOutcome
// buckets latency by this.
func (o *Orchestrator) phaseArchetype(phase string) string {
	if spec, ok := o.specFor(Phase(phase)); ok {
		return string(spec.RoleOrDefault())
	}
	return string(phasespec.PhaseSpec{Name: phase}.RoleOrDefault())
}

// builtinControlSpec supplies control-phase metadata (debugger's signal-driven
// successor) for phases with no registry home; a registry entry of the same
// name overrides it.
func builtinControlSpec(p Phase) (phasespec.PhaseSpec, bool) {
	if p == PhaseDebugger {
		return phasespec.PhaseSpec{
			Name:              string(PhaseDebugger),
			BranchingStrategy: phasespec.BranchingSignal,
			Recovery: &phasespec.RecoveryMap{Targets: map[string]string{
				"RESHIP":      string(PhaseShip),
				"RERUN_PHASE": string(PhaseAudit),
				"BLOCK":       string(PhaseEnd),
			}},
		}, true
	}
	return phasespec.PhaseSpec{}, false
}

// successorStrategy resolves branching_strategy from the phase's descriptor,
// degrading to literalSuccessorStrategy on a catalog miss: config selects
// among branches the state machine already deems legal, it never invents one.
func (o *Orchestrator) successorStrategy(p Phase) string {
	if spec, ok := o.specFor(p); ok && spec.BranchingStrategy != "" {
		return spec.BranchingStrategy
	}
	return literalSuccessorStrategy(p)
}

// literalSuccessorStrategy is the unconfigured backstop: retro is
// history-driven, debugger is signal-driven, every other phase is
// verdict-driven.
func literalSuccessorStrategy(p Phase) string {
	switch p {
	case PhaseRetro:
		return phasespec.BranchingHistory
	case PhaseDebugger:
		return phasespec.BranchingSignal
	}
	return ""
}

// PhaseMinter registers a minted phase config into a dispatchable runner.
// Implementations must return a spec with Optional=true: a minted phase can
// never satisfy or displace the build→audit→ship floor.
type PhaseMinter interface {
	Register(cfg phaseconfig.PhaseConfig) (phasespec.PhaseSpec, PhaseRunner, error)
}

// CycleRequest is the operator-facing input to RunCycle.
type CycleRequest struct {
	ProjectRoot string
	GoalHash    string
	// Env is propagated to every PhaseRequest.Env; phases consult it for
	// CLI/model selection. Copied so post-RunCycle mutation never affects an
	// in-flight or completed run.
	Env map[string]string
	// Context seeds the PhaseRequest.Context every phase receives (e.g. ship
	// reads Context["commit_message"]). Copied like Env.
	Context map[string]string
	// DisableWorkspaceGuard skips the pre-cycle workspace archive that evicts
	// stale phase artifacts from a prior interrupted cycle.
	DisableWorkspaceGuard bool
	// BypassPolicy skips policy.json pin enforcement for every phase in this
	// cycle, threaded to PhaseRequest.BypassPolicy at dispatch.
	BypassPolicy bool
}

// legacyExplanationTestStorage is deliberately package-private: only core's
// same-package legacy unit fake can opt out of the fresh-cycle contract.
type legacyExplanationTestStorage interface {
	disableFreshExplanationContractForTest() bool
}

// Orchestrator drives one cycle through the state machine, calling a
// PhaseRunner per phase and appending ledger entries. It is pure: all
// I/O is delegated to the injected Storage and Ledger ports.
type Orchestrator struct {
	storage Storage
	ledger  Ledger
	runners map[Phase]PhaseRunner
	sm      *StateMachine
	now     func() time.Time
	outcome *outcome.Recorder       // the phase-outcome recording chokepoint
	diag    *failurediag.Writer     // the failure-diag sidecar + delivery classifier
	carry   *carryover.Lifecycle    // the carryover-todo lifecycle
	learn   *failurelearning.Engine // the failure-learning engine: recorder, floor, recurrence closure
	// gitHEAD returns the current git HEAD SHA; it is NOT evidence that THIS
	// cycle shipped (a fleet sibling can move HEAD too), so the outcome label
	// reads the ship latch instead. Errors are swallowed as "no movement".
	gitHEAD func() (string, error)

	gitMutationLock gitMutationLocker

	// dossierCommit selects whether the closeout dossier is git-committed; see WithDossierCommit.
	dossierCommit bool

	// gitDirtyPaths returns the set of modified tracked paths in the main
	// repo's working directory; the tree-diff guard snapshots this before and
	// after each source-writing phase to catch a leak that escaped the sandbox.
	gitDirtyPaths func(ctx context.Context, repoRoot string) ([]string, error)

	// catalogRefresh optionally refreshes the live model catalog at cycle
	// start; best-effort, errors WARN and never block the cycle.
	catalogRefresh func(ctx context.Context) error

	// catalogRefreshStage optionally reports the resolved catalog.refresh_stage
	// stamped into the per-cycle catalog_refresh ledger entry; nil leaves the
	// stamped stage empty rather than a guess.
	catalogRefreshStage func() string

	// modelCatalogLookup is the optional live model-catalog resolvability check
	// injected into router.ClampPlanModelRouting; nil skips the resolvability
	// gate (guardrail validation still runs).
	modelCatalogLookup func(cli, tier string) (string, bool)

	// directivesProvider returns the runtime operator-directives snapshot for a
	// cycle; fail-open (a possibly-empty Set, never an error). nil = no directives.
	directivesProvider func(ctx context.Context, cycle int) directives.Set

	// compositionSnapshot, compositionGateRunner, and compositionVerdictWriter
	// wire the RUNG 0 trivial-rebase composition-verdict fast path into
	// recoverFromShipError's clean fleet-rebase branch. All three nil ⇒ the
	// fast path never fires.
	compositionSnapshot      func(ctx context.Context, worktree, runID string) (CompositionAuditSnapshot, error)
	compositionGateRunner    func(ctx context.Context, worktree string) map[string]string
	compositionVerdictWriter func(ledgerPath string, in CompositionVerdictInput) error
	laneMenu                 LaneMenuFn

	// scopedMergeReviewer wires the merge ladder's RUNG 2 scoped merge review
	// into recoverFromShipError, between the RUNG 0 carry-forward miss and the
	// RUNG 3 full re-audit. nil ⇒ RUNG 2 stays dark.
	scopedMergeReviewer ScopedMergeReviewer

	hostEffects HostEffects

	// worktree provisions/cleans the per-cycle source worktree.
	worktree WorktreeProvisioner

	// cfg + strategy drive dynamic phase routing ("model proposes, kernel
	// disposes"). The zero value (Stage:Off, StaticPreset) reproduces the
	// legacy static-state-machine behavior byte-for-byte.
	cfg      config.RoutingConfig
	strategy router.RoutingStrategy

	// planner produces the upfront whole-cycle plan. nil ⇒ no advisor plan ⇒
	// the kernel floor falls back to the never-skip spine. Consulted once at
	// cycle start, only at Stage>=Advisory, and always clamped to the
	// integrity floor before being threaded into routing.
	planner router.Planner

	// catalog is the merged phase catalog (built-in + user overlays), letting
	// the orchestrator accept and run user-defined phases without hardcoding
	// them into the Phase enum. Empty ⇒ only built-in phases exist.
	catalog phasespec.Catalog

	// safetyViolations is the ValidateSafetyInvariants result computed once at
	// construction over the wired SM + cfg + catalog. Non-empty ⇒ the loaded
	// transition config could ship without the floor; RunCycle /
	// RunCycleFromPhase fail closed with ErrUnsafeConfig before any phase runs.
	safetyViolations []string

	// registrar mints advisor-proposed phases at cycle start. nil ⇒ MintPhases
	// are ignored.
	registrar PhaseMinter

	// catalogPublisher is notified with the LIVE catalog every time a mid-cycle
	// mint changes it, so a consumer that bound a resolver over the cycle-start
	// catalog value can re-bind. nil ⇒ no-op.
	catalogPublisher func(phasespec.Catalog)

	// kb is the knowledge-base recall port: at plan time the orchestrator looks
	// up prior lessons matching the most recent failure and threads them into
	// the advisor's prompt. nil ⇒ no recall added.
	kb research.KB

	// shipFloor is the resolved integrity floor: the phases a plan reaching
	// ship must run. Empty ⇒ router.DefaultShipFloor ({tdd,build,audit}). The
	// router self-seals the non-removable evaluator regardless, so this can
	// only relax build/tdd.
	shipFloor []string

	// retryConfig is resolved once from policy.json at the composition root.
	retryConfig policy.RetryConfig

	// workflowConfig is resolved once from policy.json at the composition root.
	workflowConfig policy.WorkflowConfig

	// continuationFor resolves the continuation binding for a cycle's scope —
	// claimed, or the pinned lane-scope todo ids for a wave-planner lane. nil =
	// continuations never adopt.
	continuationFor func(projectRoot string, cycle int, scopeIDs []string) *continuation.Continuation
	// scopePathFor resolves a scoped task id to its LIVE inbox record's
	// absolute path ("" = not inbox-backed / not pending).
	scopePathFor func(projectRoot, taskID string) string
	// acsPredicates lists the cycle's ACS predicate names for the Task
	// Contract; nil ⇒ listACSPredicates (real `go test -list`).
	acsPredicates predicateLister

	// chronicle is the resolved chronicle policy (digest stage/caps), resolved
	// once from policy.json at the composition root.
	chronicle policy.ChronicleConfig

	// failureCountFor reads an inbox item's durable failure_count for retry
	// tier escalation; nil = escalation disabled.
	failureCountFor func(id string) int

	// failurePolicy is the resolved system-failure decision policy: the
	// category→action map plus the Go-enforced floor.
	failurePolicy policy.SystemFailurePolicy

	// retryAdjudicator proposes how to dispose of an audit FAIL among the
	// actions the deterministic policy already made legal. nil is a supported
	// production state: policy alone is sufficient authority to grant a retry.
	retryAdjudicator RetryAdjudicator

	// maxPhaseIterations bounds RunCycle's dispatch loop. 0 ⇒
	// defaultMaxPhaseIterations.
	maxPhaseIterations int

	// explanationContractVersion is the writer version stamped on fresh
	// cycles. It has no exported option: production callers cannot downgrade
	// the mandatory contract.
	explanationContractVersion int

	// reviewer adjudicates a finished phase's deliverable before the cycle
	// advances. nil ⇒ noopReviewer: every non-error, non-SKIPPED verdict is
	// recorded as a success.
	reviewer DeliverableReviewer

	// observer is the per-phase stall detector. Start is called once before
	// each runner.Run; the returned cancel runs once after. nil ⇒ noopObserver.
	observer Observer

	// failureAdviser is the LLM escalation tail consulted by
	// adviseOnUnclassifiedFailure, only at cfg.PhaseRecovery == StageEnforce,
	// only for unclassified artifact-timeout panes. nil ⇒ hook inert.
	failureAdviser FailureAdviser

	// contractVerifier is the breaker-neutral deliverable re-check used by the
	// correction ladder's salvage rung. nil ⇒ the salvage rung gets zero
	// budget and the ladder degrades to redispatch-only.
	contractVerifier ContractVerifier

	// throughputRecorder observes shipped cycles' coverage-floor counts for
	// the triage-capacity window. nil ⇒ no-op.
	throughputRecorder ThroughputRecorder

	// signals is the Signal Center the orchestrator listens to; nil is the
	// Null Object. signalSummary is the current cycle's view, guarded by
	// signalMu — the first production mutex in core: the Center holds no lock
	// while delivering, observeSignal never emits, and no orchestrator path
	// holds signalMu across an Emit.
	signals       *signalcenter.Center
	signalMu      sync.Mutex
	signalSummary *signalcenter.Summary

	// verdictCacheLookupHook, if non-nil, is invoked during the verdict-cache
	// lookup phase of RunCycle, letting tests verify whether the cache was
	// queried, skipped, or matched.
	verdictCacheLookupHook func(sha string, skipped bool, matched bool, entry verdictcache.Entry)

	// currentRunID holds the in-flight run's ULID as a string; the
	// construction-time stampingLedger reads it atomically on every Append.
	// Empty ⇒ no run in flight ⇒ entries are not stamped.
	currentRunID atomic.Value
}

// Option customizes an Orchestrator at construction (functional-options DI).
// Absent any option, the orchestrator runs in legacy Stage:Off mode.
type Option func(*Orchestrator)

// WithRouting injects the loaded routing config and the strategy selected
// once at the composition root. A nil strategy is ignored, leaving the
// StaticPreset default.
func WithRouting(cfg config.RoutingConfig, strategy router.RoutingStrategy) Option {
	return func(o *Orchestrator) {
		o.cfg = cfg
		if strategy != nil {
			o.strategy = strategy
		}
	}
}

// WithPlanner injects the whole-cycle phase planner. A nil planner is
// ignored: the orchestrator consults it only at Stage>=Advisory and always
// clamps its output to the integrity floor.
func WithPlanner(p router.Planner) Option {
	return func(o *Orchestrator) {
		if p != nil {
			o.planner = p
		}
	}
}

// WithCatalog injects the merged phase catalog so the orchestrator can accept
// and run user-defined (non-built-in) phases on the dynamic-routing path.
func WithCatalog(cat phasespec.Catalog) Option {
	return func(o *Orchestrator) { o.catalog = cat }
}

// WithRegistrar injects the phase minter so the orchestrator can register
// advisor-proposed phases at cycle start. Nil is ignored, leaving the no-mint default.
func WithRegistrar(m PhaseMinter) Option {
	return func(o *Orchestrator) {
		if m != nil {
			o.registrar = m
		}
	}
}

// WithCatalogPublisher injects the sink notified with the orchestrator's live
// catalog whenever a mid-cycle mint changes it, so a resolver bound over the
// cycle-start catalog can re-bind. Nil is ignored.
func WithCatalogPublisher(fn func(phasespec.Catalog)) Option {
	return func(o *Orchestrator) {
		if fn != nil {
			o.catalogPublisher = fn
		}
	}
}

// CatalogPublisherWired reports whether the composition root bound a catalog
// publisher; without it a mid-cycle mint never reaches the live contract resolver.
func (o *Orchestrator) CatalogPublisherWired() bool { return o.catalogPublisher != nil }

// WithKB injects the knowledge-base recall port. Nil is ignored, leaving the
// no-recall default.
func WithKB(kb research.KB) Option {
	return func(o *Orchestrator) {
		if kb != nil {
			o.kb = kb
		}
	}
}

// WithShipFloor sets the resolved integrity floor — the phases a plan
// reaching ship must run. Empty/nil is ignored, leaving router.DefaultShipFloor.
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

// WithContinuationResolver injects the scope continuation lookup: claimed
// scopes first, then the cycle's pinned lane-scope todo ids. Nil is ignored —
// adoption stays off.
func WithContinuationResolver(fn func(projectRoot string, cycle int, scopeIDs []string) *continuation.Continuation) Option {
	return func(o *Orchestrator) {
		if fn != nil {
			o.continuationFor = fn
		}
	}
}

// ScopePathProbe reports whether a scope-path resolver is wired and, when it
// is, what it resolves taskID to. A wiring probe, not a workflow API.
func (o *Orchestrator) ScopePathProbe(projectRoot, taskID string) (string, bool) {
	if o.scopePathFor == nil {
		return "", false
	}
	return o.scopePathFor(projectRoot, taskID), true
}

// WithScopePathResolver injects the live-record path lookup for scoped task
// ids, so a lane's phases receive the one correct file instead of a bare
// name that could resolve to a stale duplicate.
func WithScopePathResolver(fn func(projectRoot, taskID string) string) Option {
	return func(o *Orchestrator) {
		if fn != nil {
			o.scopePathFor = fn
		}
	}
}

// WithWorkflowConfig injects the resolved workflow policy.
func WithWorkflowConfig(cfg policy.WorkflowConfig) Option {
	return func(o *Orchestrator) { o.workflowConfig = cfg }
}

// WithChronicleConfig injects the resolved chronicle policy (recent-outcomes
// digest stage + caps). The zero-option default is digest=shadow.
func WithChronicleConfig(cfg policy.ChronicleConfig) Option {
	return func(o *Orchestrator) { o.chronicle = cfg }
}

// WithFailureCountReader injects the item failure-count read seam. Nil is
// ignored, preserving any prior reader.
func WithFailureCountReader(fn func(id string) int) Option {
	return func(o *Orchestrator) {
		if fn != nil {
			o.failureCountFor = fn
		}
	}
}

// WithFailurePolicy injects the resolved system-failure decision policy. The
// zero-option default is the compiled DefaultSystemFailurePolicy.
func WithFailurePolicy(fp policy.SystemFailurePolicy) Option {
	return func(o *Orchestrator) { o.failurePolicy = fp }
}

// WithRetryAdjudicator injects the audit-FAIL adjudication Strategy. Absent, the
// deterministic policy decides alone.
func WithRetryAdjudicator(a RetryAdjudicator) Option {
	return func(o *Orchestrator) { o.retryAdjudicator = a }
}

// WithMaxPhaseIterations overrides the dispatch-loop iteration bound. n<=0 is
// ignored so the defaultMaxPhaseIterations safety oracle stands.
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

// WithWorktreeBase injects the operator worktree-base override. Empty is
// ignored, leaving the gitWorktree default. Mutually exclusive with
// WithWorktreeProvisioner (both set o.worktree); production uses only this
// one, tests use only the fake.
func WithWorktreeBase(base string) Option {
	return func(o *Orchestrator) {
		if base != "" {
			o.worktree = gitWorktree{baseOverride: base}
		}
	}
}

// WithObserver injects a per-phase stall detector. The orchestrator calls
// observer.Start before each phase's runner.Run and the returned cancel
// after. A nil observer keeps the noopObserver default.
func WithObserver(o Observer) Option {
	return func(orch *Orchestrator) {
		if o != nil {
			orch.observer = o
		}
	}
}

// WithCatalogRefresher injects a best-effort live-model-catalog refresh run at
// cycle start. The closure owns its TTL/staleness check; the orchestrator
// calls it once per cycle before any phase runs and only WARNs on error.
func WithCatalogRefresher(fn func(ctx context.Context) error) Option {
	return func(o *Orchestrator) { o.catalogRefresh = fn }
}

// WithCatalogRefreshStage injects the resolved catalog.refresh_stage accessor
// stamped into the per-cycle catalog_refresh ledger entry. Optional: without
// it the entry still stamps with an empty stage rather than a guess.
func WithCatalogRefreshStage(fn func() string) Option {
	return func(o *Orchestrator) { o.catalogRefreshStage = fn }
}

// WithModelCatalogLookup injects the model resolvability check consulted by
// router.ClampPlanModelRouting: (cli,tier)→(model,ok). Nil skips the
// resolvability gate — the plan's guardrail validation still applies.
func WithModelCatalogLookup(fn func(cli, tier string) (string, bool)) Option {
	return func(o *Orchestrator) { o.modelCatalogLookup = fn }
}

// WithDirectivesProvider injects the runtime operator-directives provider.
// The closure re-reads the directive files each cycle so live operator edits
// propagate at the next cycle boundary; a nil fn leaves directives off.
func WithDirectivesProvider(fn func(ctx context.Context, cycle int) directives.Set) Option {
	return func(o *Orchestrator) { o.directivesProvider = fn }
}

// WithReviewer injects a per-phase deliverable reviewer. The orchestrator
// calls reviewer.Review after each phase's runner.Run returns a non-error,
// non-SKIPPED verdict, before the ledger append. Approve=false aborts the
// cycle with the reviewer's Reason.
func WithReviewer(r DeliverableReviewer) Option {
	return func(o *Orchestrator) {
		if r != nil {
			o.reviewer = withMandatoryExplanationReviewer(r)
		}
	}
}

// WithContractVerifier injects the breaker-neutral deliverable re-check the
// correction ladder's salvage rung verifies relocations with. Nil is
// ignored, leaving the redispatch-only ladder.
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

// WithVerdictCacheLookupHook registers a hook that is called when the verdict cache is checked.
func WithVerdictCacheLookupHook(fn func(sha string, skipped bool, matched bool, entry verdictcache.Entry)) Option {
	return func(o *Orchestrator) {
		o.verdictCacheLookupHook = fn
	}
}

// HasRunner reports whether a PhaseRunner is registered for p. It is the
// composition-root's read seam: a phase the router can nominate but that has
// no runner is silently skipped by cyclerun_dispatch's missing-runner escape
// hatch, so tests assert on this to prove the routing→dispatch handoff is
// actually wired, not just that Route() names it.
func (o *Orchestrator) HasRunner(p Phase) bool {
	_, ok := o.runners[p]
	return ok
}

// NewOrchestrator wires the orchestrator with its dependencies. Routing stays
// off unless a WithRouting option supplies an enabled-stage config.
func NewOrchestrator(storage Storage, ledger Ledger, runners map[Phase]PhaseRunner, opts ...Option) *Orchestrator {
	o := &Orchestrator{
		storage:                    storage,
		ledger:                     ledger,
		runners:                    runners,
		sm:                         NewStateMachine(),
		now:                        time.Now,
		gitHEAD:                    defaultGitHEAD,
		gitMutationLock:            defaultGitMutationLock,
		dossierCommit:              true,
		gitDirtyPaths:              defaultGitDirtyPaths,
		worktree:                   gitWorktree{},
		strategy:                   router.StaticPreset{},
		retryConfig:                policy.Policy{}.RetryConfig(),
		workflowConfig:             policy.Policy{}.WorkflowConfig(),
		chronicle:                  policy.Policy{}.ChronicleConfig(),
		failurePolicy:              policy.DefaultSystemFailurePolicy(),
		reviewer:                   withMandatoryExplanationReviewer(noopReviewer{}),
		explanationContractVersion: explanationdocs.CurrentContractVersion,
		observer:                   noopObserver{},
	}
	if legacy, ok := storage.(legacyExplanationTestStorage); ok && legacy.disableFreshExplanationContractForTest() {
		o.explanationContractVersion = 0
	}
	for _, opt := range opts {
		opt(o)
	}
	// The recorder reads the clock and the catalog live: tests swap o.now
	// after construction, and mints change the catalog mid-cycle.
	o.outcome = o.wiredRecorder()
	o.diag = o.wiredFailureDiag()
	o.carry = o.wiredCarryover()
	o.learn = o.wiredFailureLearning() // after o.carry: the engine mints through the one wired lifecycle
	// Hand the state machine its config-driven verdict-branch resolution now
	// that the catalog (hence specFor) is settled by options; an empty catalog
	// or spine order degrades to the literal table.
	o.sm.WithCatalog(o.specFor).WithSpine(spinePhasesFrom(o.cfg.SpineOrder)).
		WithLegalGraph(legalGraphFrom(o.cfg.LegalSuccessors))
	// Computed once over the fully-wired SM; RunCycle/RunCycleFromPhase fail
	// closed if it found a floor hole.
	o.safetyViolations = ValidateSafetyInvariants(o.sm, o.cfg, o.catalog)
	// The run-id stamping decorator wraps the ledger exactly once at
	// construction; the per-run identity flows through the atomic
	// currentRunID so goroutine-spawning observers can read it race-free.
	o.ledger = stampingLedger{inner: o.ledger, runID: &o.currentRunID}
	return o
}

func fleetMode(env map[string]string) bool {
	return envchain.BoolValue(env[ipcenv.FleetKey], false)
}

// ensureSafeConfig fails closed when the loaded transition config violates a
// safety invariant: the composition root computes the violations at
// construction, and every cycle-run entry refuses to proceed if the floor
// could be bypassed, so an unsafe registry edit can never run a single phase.
func (o *Orchestrator) ensureSafeConfig() error {
	if len(o.safetyViolations) > 0 {
		return fmt.Errorf("%w: %s", ErrUnsafeConfig, strings.Join(o.safetyViolations, "; "))
	}
	return nil
}

// RunCycle drives one cycle from PhaseStart to PhaseEnd, returning a summary
// of what ran. The lock is acquired up front (except in fleet mode, see
// fleetMode) and released on every exit path. State is updated incrementally
// so a crash leaves an inspectable trail in .evolve/.
func (o *Orchestrator) RunCycle(ctx context.Context, req CycleRequest) (_ CycleResult, retErr error) {
	if err := o.ensureSafeConfig(); err != nil {
		return CycleResult{}, err
	}
	// newCycleRun does resource setup (lock, state read, cycle allocation,
	// run-ID mint, CycleState, workspace-pollution guard, source worktree,
	// cycle-state persist, run lease) and returns a cleanup closure carrying
	// the exit actions; RunCycle defers it in its own frame so the actions
	// fire here at cycle exit, LIFO.
	init, cleanup, err := o.newCycleRun(ctx, req)
	if err != nil {
		return CycleResult{}, err
	}
	// cycleRun is the one addressable home for the dispatch loop's shared and
	// loop-carried state, so late mutations by sub-methods (pointer receivers
	// throughout) are visible to the exit defers and the next iteration.
	cr := &cycleRun{
		o:                 o,
		ctx:               ctx,
		req:               req,
		cycle:             init.cycle,
		mainDirtyBaseline: init.mainDirtyBaseline,
		consoleLeased:     init.consoleLeased,
		state:             init.state,
		cs:                init.cs,
		result:            CycleResult{Cycle: init.cycle, FinalVerdict: VerdictPASS},
		current:           PhaseStart,
		lastVerdict:       VerdictPASS,
		retryConfig:       o.retryConfig,
		workflowConfig:    o.workflowConfig,
	}
	// Registered first (fires last, LIFO); reads cr.preserveWorktree /
	// cr.cycleCompletedNormally at defer-execution time so late mutations by
	// the dispatch loop are honored.
	defer func() { cleanup(cr.preserveWorktree, cr.cycleCompletedNormally) }()
	// No exit path may leave the ship-window lease held — siblings would wait
	// out the full TTL. Idempotent; the normal release happens in recordAndBranch.
	defer cr.releaseShipWindow()
	// No exit path may leave a started cycle without its evidence trail
	// (dossier + digest + coherent state). retErr is read at defer-execution
	// time as the abort's cause.
	defer func() { cr.abnormalEpilogue(retErr) }()
	// A graceful interrupt must resume the phase that was actually active, not
	// the previous phase-complete boundary. Registered after abnormalEpilogue so
	// it runs first and captures cs.Phase before the epilogue marks it aborted.
	defer func() { cr.checkpointInterruptedPhase(retErr) }()
	// A marker created at fresh-cycle allocation distinguishes new cycles,
	// whose Build explanation contract is mandatory, from old workspaces being
	// resumed after an upgrade. Persist it before the first phase so a crash
	// before Build cannot accidentally turn a new cycle into a legacy exemption.
	if cr.cs.ExplanationDocumentationVersion != 0 {
		if err := activateBuildExplanationContract(req.ProjectRoot, cr.cs); err != nil {
			return CycleResult{}, fmt.Errorf("activate Build explanation contract: %w", err)
		}
	}

	// planCycle resolves catalog refresh, per-cycle env/ctx snapshots, fleet
	// scope, the challenge token, pre-cycle HEAD, and the clamped whole-cycle
	// advisory plan; outputs thread into every routing decision below.
	plan := o.planCycle(ctx, req, cr.state, cr.cs, cr.cycle)
	cr.envSnap = plan.envSnap
	cr.ctxSnap = plan.ctxSnap
	cr.preCycleHEAD = plan.preCycleHEAD
	cr.cs.PreCycleHEAD = plan.preCycleHEAD
	cr.benchedCLIs = plan.benchedCLIs
	cr.clampedPlan = plan.clampedPlan
	cr.directivesSet = plan.directivesSet

	// Runs even when RunCycle returns an error, so partial timing data is
	// preserved for operator inspection; reads cr.phaseTimings live.
	defer func() {
		if len(cr.phaseTimings) > 0 {
			cr.flushPhaseTimings()
		}
		// Best-effort, abort paths included: an interaction not recorded with
		// its outcome doesn't exist.
		if werr := interaction.WriteRollup(cr.cs.WorkspacePath); werr != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN interaction-summary write: %v\n", werr)
		}
		// o.catalog is read at defer-exec time, so mid-cycle mints are visible
		// to the resolver.
		emitPhaseOutputsSignal(cr.cs.WorkspacePath, cr.cycle, cr.cs.CompletedPhases,
			phasecontract.NewCatalogResolver(cr.o.catalog.Get))
	}()
	// Content-addressed audit-reuse probe (SHADOW): observe-only, logs a
	// would-reuse and changes nothing. Pre-loop on purpose, targeting a
	// preserved/re-dispatched worktree that still carries prior-audited
	// content; a fresh cycle's clean worktree never matches.
	if cr.cs.ActiveWorktree != "" {
		if sha := worktreeContentSHA(ctx, cr.req.ProjectRoot, cr.cs.ActiveWorktree, cr.cs.WorkspacePath); sha != "" {
			baseTree := worktreeBaseTreeSHA(ctx, cr.cs.ActiveWorktree, cr.cs.WorktreeBaseSHA)
			if !verdictcache.ProbeEligible(baseTree, sha) {
				// An untouched/fresh worktree skips the lookup to prevent
				// fresh-base collisions.
				if o.verdictCacheLookupHook != nil {
					o.verdictCacheLookupHook(sha, true, false, verdictcache.Entry{})
				}
			} else {
				e, ok := verdictcache.NewStore(req.ProjectRoot, o.now).Lookup(sha)
				if o.verdictCacheLookupHook != nil {
					o.verdictCacheLookupHook(sha, false, ok, e)
				}
				if ok {
					fmt.Fprintf(os.Stderr, "[verdict-cache SHADOW] worktree tree_sha=%s matched cycle=%d verdict=%s — would skip tdd/build/audit (ADR-0048 Slice B; enforce pending)\n", sha, e.Cycle, e.Verdict)
				}
			}
		}
	}

	// Bounded loop guards against any transition-table cycle bug. Labeled so
	// the extracted sub-methods can signal loop termination (loopBreak →
	// `break OuterLoop`) from inside the switch ladder below — a bare `break`
	// there would exit the switch, not the loop.
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
		// Triage continuation adoption runs at the previous phase boundary and
		// may replace both worktree and review base. Seal that final context once
		// before the first Build dispatch; repeats are idempotent.
		if next == PhaseBuild {
			if err := sealBuildExplanationContext(req.ProjectRoot, cr.cs); err != nil {
				return cr.result, fmt.Errorf("seal Build explanation context: %w", err)
			}
		}

		// At ParallelEvaluate=enforce, when `next` begins a run of independent
		// post-build checking phases (archetype "evaluate", audit excluded),
		// dispatch the whole run concurrently as one batch and skip the
		// per-phase path. StageOff/Shadow never enter here.
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

		// A configured deterministic gate that FAILed gets one bounded builder
		// fix and a same-gate re-run before the verdict is recorded. Nothing
		// downstream is bypassed — the same gate must pass and audit/ship
		// floors run unchanged.
		if act, merr := cr.maybeRemediate(next, &dr); act == loopAbort {
			return cr.result, merr
		}

		// End-of-iteration record + branch (success ledger, bindings,
		// CompletedPhases persist, checkpoint, outcome record + cursor advance,
		// retro/debugger non-verdict-driven branches) → recordAndBranch.
		switch act, berr := cr.recordAndBranch(next, dr); act {
		case loopAbort:
			return cr.result, berr
		case loopBreak:
			cr.reachedPhaseEnd = true
			break OuterLoop
		}

		// The post-scout re-plan hook fires once per cycle after scout's
		// handoff is recorded (above) and before the next selectNext, so a
		// re-plan can never widen the run-set or bypass the spine gate.
		if next == PhaseScout {
			normalizeScoutGoalHash(cr.cs.WorkspacePath)
			cr.postScoutReplan()
		}
		// Adoption is keyed to the actual claim: triage claims items into
		// processing/cycle-N mid-cycle, so only after the triage phase can the
		// resolver see whether this cycle's scope carries preserved work.
		if next == PhaseTriage {
			if err := cr.adoptContinuationAfterTriage(); err != nil {
				cr.result.FinalVerdict = VerdictFAIL
				return cr.result, err
			}
		}
	}

	// The bounded loop can exit by exhausting its iteration budget instead of
	// reaching PhaseEnd; record an explicit abort naming the stalled phase so
	// the escape is diagnosable rather than FAILED_UNEXPLAINED. Runs before
	// finalizeCycle so the recorded FAIL preserves the worktree for salvage.
	if !cr.reachedPhaseEnd {
		cr.recordChokepointEscape(fmt.Sprintf(
			"transition-table cycle guard: dispatch loop ran %d iterations without reaching PhaseEnd (cursor stalled at phase %q) — a transition cycle prevented termination; ADR-0044 C1 chokepoint escape",
			maxIter, cr.current))
	}

	if err := cr.completeCycle(); err != nil {
		return cr.result, err
	}

	return cr.result, nil
}

// WithDossierCommit decides whether each cycle's closeout dossier is
// git-committed into the project root (the production default) or only
// written — the --simulate root's contract, since a no-LLM plumbing walk
// must never mutate the operator's repository.
func WithDossierCommit(commit bool) Option {
	return func(o *Orchestrator) { o.dossierCommit = commit }
}
