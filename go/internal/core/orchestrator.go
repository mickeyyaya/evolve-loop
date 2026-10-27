package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
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
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/verdictcache"
)

var PhaseBoundaryCheckpointer func(cs CycleState, projectRoot string, now time.Time) error

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

func (o *Orchestrator) skippableOptional(p Phase) (phasespec.PhaseSpec, bool) {
	if isConfiguredMandatory(o.cfg, string(p)) {
		return phasespec.PhaseSpec{}, false
	}
	spec, ok := o.catalog.Get(string(p))
	if !ok || !spec.Optional || slices.Contains(o.resolvedShipFloor(), string(p)) {
		return phasespec.PhaseSpec{}, false
	}
	return spec, true
}

func (o *Orchestrator) optionalInfraSkip(p Phase, err error) bool {
	if !IsOptionalSkippableError(err) {
		return false
	}
	_, ok := o.skippableOptional(p)
	return ok
}

func (o *Orchestrator) postShipObserverSkip(p Phase, shipped bool) bool {
	if !shipped || p == PhaseShip {
		return false
	}
	spec, ok := o.skippableOptional(p)
	return ok && spec.RoleOrDefault() == phasespec.RoleControl
}

func (o *Orchestrator) specFor(p Phase) (phasespec.PhaseSpec, bool) {
	if spec, ok := o.catalog.Get(canonicalCatalogName(p)); ok {
		return spec, true
	}
	return builtinControlSpec(p)
}

func (o *Orchestrator) phaseArchetype(phase string) string {
	if spec, ok := o.specFor(Phase(phase)); ok {
		return string(spec.RoleOrDefault())
	}
	return string(phasespec.PhaseSpec{Name: phase}.RoleOrDefault())
}

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

func (o *Orchestrator) successorStrategy(p Phase) string {
	if spec, ok := o.specFor(p); ok && spec.BranchingStrategy != "" {
		return spec.BranchingStrategy
	}
	return literalSuccessorStrategy(p)
}

func literalSuccessorStrategy(p Phase) string {
	switch p {
	case PhaseRetro:
		return phasespec.BranchingHistory
	case PhaseDebugger:
		return phasespec.BranchingSignal
	}
	return ""
}

type PhaseMinter interface {
	Register(cfg phaseconfig.PhaseConfig) (phasespec.PhaseSpec, PhaseRunner, error)
}

type CycleRequest struct {
	ProjectRoot           string
	GoalHash              string
	Env                   map[string]string
	Context               map[string]string
	DisableWorkspaceGuard bool
	BypassPolicy          bool
}

type legacyExplanationTestStorage interface {
	disableFreshExplanationContractForTest() bool
}

type Orchestrator struct {
	storage Storage
	ledger  Ledger
	runners map[Phase]PhaseRunner
	sm      *StateMachine
	now     func() time.Time
	outcome *outcome.Recorder
	diag    *failurediag.Writer
	carry   *carryover.Lifecycle
	learn   *failurelearning.Engine
	gitHEAD func() (string, error)

	resumeDispatch func(cr *cycleRun, next Phase, req PhaseRequest, hooks retryOpts) (map[string]bool, PhaseResponse, int, error)

	gitMutationLock gitMutationLocker

	dossierDestination DossierDestination

	gitDirtyPaths func(ctx context.Context, repoRoot string) ([]string, error)

	catalogRefresh func(ctx context.Context) error

	catalogRefreshStage func() string

	modelCatalogLookup func(cli, tier string) (string, bool)

	directivesProvider func(ctx context.Context, cycle int) directives.Set

	compositionSnapshot      func(ctx context.Context, worktree, runID string) (CompositionAuditSnapshot, error)
	compositionGateRunner    func(ctx context.Context, worktree string) map[string]string
	compositionVerdictWriter func(ledgerPath string, in CompositionVerdictInput) error
	laneMenu                 LaneMenuFn
	protectedSurface         func(string) bool

	scopedMergeReviewer ScopedMergeReviewer

	hostEffects HostEffects

	stderr io.Writer

	continuationShed func(projectRoot string, cycle int, declined continuation.Continuation) ([]string, error)

	shipWindow *shipWindowHold

	worktree WorktreeProvisioner

	cfg      config.RoutingConfig
	strategy router.RoutingStrategy

	planner router.Planner

	catalog phasespec.Catalog

	safetyViolations []string

	registrar PhaseMinter

	catalogPublisher func(phasespec.Catalog)
	cliRouter        *cliroute.Router

	kb research.KB

	shipFloor []string

	retryConfig policy.RetryConfig

	workflowConfig policy.WorkflowConfig

	continuationFor func(projectRoot string, cycle int, scopeIDs []string) *continuation.Continuation
	scopePathFor    func(projectRoot, taskID string) string
	acsPredicates   predicateLister

	chronicle policy.ChronicleConfig

	failureCountFor func(id string) int
	failureClassFor func(id string) string
	rungHandlers    map[retryladder.Rung]RungHandler

	failurePolicy policy.SystemFailurePolicy

	retryAdjudicator RetryAdjudicator

	maxPhaseIterations int

	explanationContractVersion int

	reviewer DeliverableReviewer

	observer Observer

	failureAdviser FailureAdviser

	contractVerifier ContractVerifier

	throughputRecorder ThroughputRecorder

	signals       *signalcenter.Center
	signalMu      sync.Mutex
	signalSummary *signalcenter.Summary

	verdictCacheLookupHook func(sha string, skipped bool, matched bool, entry verdictcache.Entry)

	currentRunID atomic.Value
}

type Option func(*Orchestrator)

func WithRouting(cfg config.RoutingConfig, strategy router.RoutingStrategy) Option {
	return func(o *Orchestrator) {
		o.cfg = cfg
		if strategy != nil {
			o.strategy = strategy
		}
	}
}

func WithPlanner(p router.Planner) Option {
	return func(o *Orchestrator) {
		if p != nil {
			o.planner = p
		}
	}
}

func WithCatalog(cat phasespec.Catalog) Option {
	return func(o *Orchestrator) { o.catalog = cat }
}

func WithRegistrar(m PhaseMinter) Option {
	return func(o *Orchestrator) {
		if m != nil {
			o.registrar = m
		}
	}
}

func WithCatalogPublisher(fn func(phasespec.Catalog)) Option {
	return func(o *Orchestrator) {
		if fn != nil {
			o.catalogPublisher = fn
		}
	}
}

func WithCLIRouter(r *cliroute.Router) Option {
	return func(o *Orchestrator) { o.cliRouter = r }
}

func (o *Orchestrator) CLIRouterWired() bool { return o.cliRouter != nil }

func (o *Orchestrator) CatalogPublisherWired() bool { return o.catalogPublisher != nil }

func WithKB(kb research.KB) Option {
	return func(o *Orchestrator) {
		if kb != nil {
			o.kb = kb
		}
	}
}

func WithShipFloor(floor []string) Option {
	return func(o *Orchestrator) {
		if len(floor) > 0 {
			o.shipFloor = floor
		}
	}
}

func WithRetryConfig(cfg policy.RetryConfig) Option {
	return func(o *Orchestrator) { o.retryConfig = cfg }
}

func WithContinuationResolver(fn func(projectRoot string, cycle int, scopeIDs []string) *continuation.Continuation) Option {
	return func(o *Orchestrator) {
		if fn != nil {
			o.continuationFor = fn
		}
	}
}

func WithContinuationShedder(fn func(projectRoot string, cycle int, declined continuation.Continuation) ([]string, error)) Option {
	return func(o *Orchestrator) {
		if fn != nil {
			o.continuationShed = fn
		}
	}
}

func (o *Orchestrator) ContinuationShedProbe(projectRoot string, cycle int, declined continuation.Continuation) ([]string, bool, error) {
	if o.continuationShed == nil {
		return nil, false, nil
	}
	shed, err := o.continuationShed(projectRoot, cycle, declined)
	return shed, true, err
}

func (o *Orchestrator) ScopePathProbe(projectRoot, taskID string) (string, bool) {
	if o.scopePathFor == nil {
		return "", false
	}
	return o.scopePathFor(projectRoot, taskID), true
}

func WithScopePathResolver(fn func(projectRoot, taskID string) string) Option {
	return func(o *Orchestrator) {
		if fn != nil {
			o.scopePathFor = fn
		}
	}
}

func WithWorkflowConfig(cfg policy.WorkflowConfig) Option {
	return func(o *Orchestrator) { o.workflowConfig = cfg }
}

func WithChronicleConfig(cfg policy.ChronicleConfig) Option {
	return func(o *Orchestrator) { o.chronicle = cfg }
}

func WithFailureCountReader(fn func(id string) int) Option {
	return func(o *Orchestrator) {
		if fn != nil {
			o.failureCountFor = fn
		}
	}
}

func WithFailureClassReader(fn func(id string) string) Option {
	return func(o *Orchestrator) { o.failureClassFor = fn }
}

func WithFailurePolicy(fp policy.SystemFailurePolicy) Option {
	return func(o *Orchestrator) { o.failurePolicy = fp }
}

func WithRetryAdjudicator(a RetryAdjudicator) Option {
	return func(o *Orchestrator) { o.retryAdjudicator = a }
}

func WithMaxPhaseIterations(n int) Option {
	return func(o *Orchestrator) {
		if n > 0 {
			o.maxPhaseIterations = n
		}
	}
}

func WithWorktreeProvisioner(p WorktreeProvisioner) Option {
	return func(o *Orchestrator) {
		if p != nil {
			o.worktree = p
		}
	}
}

func WithWorktreeBase(base string) Option {
	return func(o *Orchestrator) {
		if base != "" {
			o.worktree = gitWorktree{baseOverride: base}
		}
	}
}

func WithObserver(o Observer) Option {
	return func(orch *Orchestrator) {
		if o != nil {
			orch.observer = o
		}
	}
}

func WithCatalogRefresher(fn func(ctx context.Context) error) Option {
	return func(o *Orchestrator) { o.catalogRefresh = fn }
}

func WithCatalogRefreshStage(fn func() string) Option {
	return func(o *Orchestrator) { o.catalogRefreshStage = fn }
}

func WithModelCatalogLookup(fn func(cli, tier string) (string, bool)) Option {
	return func(o *Orchestrator) { o.modelCatalogLookup = fn }
}

func WithDirectivesProvider(fn func(ctx context.Context, cycle int) directives.Set) Option {
	return func(o *Orchestrator) { o.directivesProvider = fn }
}

func WithReviewer(r DeliverableReviewer) Option {
	return func(o *Orchestrator) {
		if r != nil {
			o.reviewer = withMandatoryExplanationReviewer(r)
		}
	}
}

func WithContractVerifier(v ContractVerifier) Option {
	return func(o *Orchestrator) {
		if v != nil {
			o.contractVerifier = v
		}
	}
}

func WithGitDirtyPaths(fn func(ctx context.Context, repoRoot string) ([]string, error)) Option {
	return func(o *Orchestrator) {
		if fn != nil {
			o.gitDirtyPaths = fn
		}
	}
}

func WithVerdictCacheLookupHook(fn func(sha string, skipped bool, matched bool, entry verdictcache.Entry)) Option {
	return func(o *Orchestrator) {
		o.verdictCacheLookupHook = fn
	}
}

func (o *Orchestrator) HasRunner(p Phase) bool {
	_, ok := o.runners[p]
	return ok
}

func NewOrchestrator(storage Storage, ledger Ledger, runners map[Phase]PhaseRunner, opts ...Option) *Orchestrator {
	o := &Orchestrator{
		storage:                    storage,
		ledger:                     ledger,
		runners:                    runners,
		sm:                         NewStateMachine(),
		shipWindow:                 &shipWindowHold{},
		now:                        time.Now,
		gitHEAD:                    defaultGitHEAD,
		resumeDispatch:             (*cycleRun).snapshotThenDispatch,
		gitMutationLock:            defaultGitMutationLock,
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
	o.outcome = o.wiredRecorder()
	o.diag = o.wiredFailureDiag()
	o.carry = o.wiredCarryover()
	o.learn = o.wiredFailureLearning()
	o.sm.WithCatalog(o.specFor).WithSpine(spinePhasesFrom(o.cfg.SpineOrder)).
		WithLegalGraph(legalGraphFrom(o.cfg.LegalSuccessors))
	o.safetyViolations = ValidateSafetyInvariants(o.sm, o.cfg, o.catalog)
	o.ledger = stampingLedger{inner: o.ledger, runID: &o.currentRunID}
	return o
}

func fleetMode(env map[string]string) bool {
	return envchain.BoolValue(env[ipcenv.FleetKey], false)
}

func (o *Orchestrator) ensureSafeConfig() error {
	if len(o.safetyViolations) > 0 {
		return fmt.Errorf("%w: %s", ErrUnsafeConfig, strings.Join(o.safetyViolations, "; "))
	}
	return nil
}

func (o *Orchestrator) RunCycle(ctx context.Context, req CycleRequest) (_ CycleResult, retErr error) {
	if err := o.ensureSafeConfig(); err != nil {
		return CycleResult{}, err
	}
	init, cleanup, err := o.newCycleRun(ctx, req)
	if err != nil {
		return CycleResult{}, err
	}
	cr := &cycleRun{
		o:              o,
		ctx:            ctx,
		req:            req,
		cycle:          init.cycle,
		consoleLeased:  init.consoleLeased,
		state:          init.state,
		cs:             init.cs,
		result:         CycleResult{Cycle: init.cycle, FinalVerdict: VerdictPASS},
		current:        PhaseStart,
		lastVerdict:    VerdictPASS,
		retryConfig:    o.retryConfig,
		workflowConfig: o.workflowConfig,
	}
	defer func() { cleanup(cr.preserveWorktree, cr.cycleCompletedNormally) }()
	defer o.releaseShipWindow()
	defer func() { cr.abnormalEpilogue(retErr) }()
	defer func() { cr.checkpointInterruptedPhase(retErr) }()
	if cr.cs.ExplanationDocumentationVersion != 0 {
		if err := activateBuildExplanationContract(req.ProjectRoot, cr.cs); err != nil {
			return CycleResult{}, fmt.Errorf("activate Build explanation contract: %w", err)
		}
	}

	plan := o.planCycle(ctx, req, cr.state, cr.cs, cr.cycle)
	cr.envSnap = plan.envSnap
	cr.ctxSnap = plan.ctxSnap
	cr.preCycleHEAD = plan.preCycleHEAD
	cr.cs.PreCycleHEAD = plan.preCycleHEAD
	o.persistResumeWitness(req.ProjectRoot, cr.cs)
	cr.benchedCLIs = plan.benchedCLIs
	cr.clampedPlan = plan.clampedPlan
	cr.directivesSet = plan.directivesSet

	defer func() {
		if len(cr.phaseTimings) > 0 {
			cr.flushPhaseTimings()
		}
		if werr := interaction.WriteRollup(cr.cs.WorkspacePath); werr != nil {
			fmt.Fprintf(o.errOut(), "[orchestrator] WARN interaction-summary write: %v\n", werr)
		}
		emitPhaseOutputsSignal(cr.cs.WorkspacePath, cr.cycle, cr.cs.CompletedPhases,
			phasecontract.NewCatalogResolver(cr.o.catalog.Get))
	}()
	if cr.cs.ActiveWorktree != "" {
		if sha := worktreeContentSHA(ctx, cr.req.ProjectRoot, cr.cs.ActiveWorktree, cr.cs.WorkspacePath); sha != "" {
			baseTree := worktreeBaseTreeSHA(ctx, cr.cs.ActiveWorktree, cr.cs.WorktreeBaseSHA)
			if !verdictcache.ProbeEligible(baseTree, sha) {
				if o.verdictCacheLookupHook != nil {
					o.verdictCacheLookupHook(sha, true, false, verdictcache.Entry{})
				}
			} else {
				e, ok := verdictcache.NewStore(req.ProjectRoot, o.now).Lookup(sha)
				if o.verdictCacheLookupHook != nil {
					o.verdictCacheLookupHook(sha, false, ok, e)
				}
				if ok {
					fmt.Fprintf(o.errOut(), "[verdict-cache SHADOW] worktree tree_sha=%s matched cycle=%d verdict=%s — would skip tdd/build/audit (ADR-0048 Slice B; enforce pending)\n", sha, e.Cycle, e.Verdict)
				}
			}
		}
	}

	maxIter := o.phaseIterationLimit()
OuterLoop:
	for safety := 0; safety < maxIter; safety++ {
		next, act, serr := cr.selectNext()
		switch act {
		case loopAbort:
			return cr.result, serr
		case loopBreak:
			cr.reachedPhaseEnd = true
			break OuterLoop
		}
		if next == PhaseBuild {
			cr.predicatesAtBuild = cyclePredicateSources(cr.cs.ActiveWorktree, cr.cycle)
			if err := sealBuildExplanationContext(req.ProjectRoot, cr.cs); err != nil {
				return cr.result, fmt.Errorf("seal Build explanation context: %w", err)
			}
		}

		if cr.o.cfg.ParallelEvaluate == config.StageEnforce {
			if batch := cr.evaluateBatchAt(next); len(batch) >= 2 {
				if bact, berr := cr.dispatchEvaluateBatch(batch); bact == loopAbort {
					return cr.result, berr
				}
				continue
			}
		}

		dr, act, derr := cr.dispatch(next)
		switch act {
		case loopAbort:
			return cr.result, derr
		case loopContinue:
			continue
		}

		switch act, rerr := cr.reviewAndGuard(next, &dr); act {
		case loopAbort:
			return cr.result, rerr
		case loopBreak:
			cr.reachedPhaseEnd = true
			break OuterLoop
		case loopContinue:
			continue
		}

		if act, merr := cr.maybeRemediate(next, &dr); act == loopAbort {
			return cr.result, merr
		}

		switch act, berr := cr.recordAndBranch(next, dr); act {
		case loopAbort:
			return cr.result, berr
		case loopBreak:
			cr.reachedPhaseEnd = true
			break OuterLoop
		}

		if next == PhaseScout {
			normalizeScoutGoalHash(cr.cs.WorkspacePath, o.errOut())
			cr.postScoutReplan()
		}
		if next == PhaseTriage {
			if err := cr.adoptContinuationAfterTriage(); err != nil {
				cr.result.FinalVerdict = VerdictFAIL
				return cr.result, err
			}
		}
	}

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

type DossierDestination int

const (
	DossierCommitted DossierDestination = iota
	DossierFilesOnly
	DossierPending
)

func WithDossierDestination(d DossierDestination) Option {
	return func(o *Orchestrator) { o.dossierDestination = d }
}
