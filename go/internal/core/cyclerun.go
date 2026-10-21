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
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/directives"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards/treediff"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

var (
	fleetIsolationSetenv = os.Setenv
	challengeTokenRead   = rand.Read
)

type loopAction int

const (
	loopNext loopAction = iota
	loopContinue
	loopBreak
	loopAbort
)

const defaultMaxPhaseIterations = 32

func (o *Orchestrator) phaseIterationLimit() int {
	if o.maxPhaseIterations <= 0 {
		return defaultMaxPhaseIterations
	}
	return o.maxPhaseIterations
}

type cycleRun struct {
	o   *Orchestrator
	ctx context.Context

	protectedRouteDiags []Diagnostic
	predicatesAtBuild   map[string]string
	lastBlockerRefusal  string

	req            CycleRequest
	cycle          int
	consoleLeased  map[string]bool
	envSnap        map[string]string
	ctxSnap        map[string]string
	preCycleHEAD   string
	benchedCLIs    []router.BenchedCLI
	clampedPlan    *router.PhasePlan
	directivesSet  directives.Set
	retryConfig    policy.RetryConfig
	workflowConfig policy.WorkflowConfig

	state           State
	cs              CycleState
	checkout        *checkoutDecision
	result          CycleResult
	phaseTimings    []phaseTimingEntry
	timingsFlushed  bool
	timingsComposed []phaseTimingEntry

	current           Phase
	lastVerdict       string
	scheduledNext     Phase
	routingSeq        int
	remediationRounds map[Phase]int
	phaseDispatches   map[Phase]int
	dispatchMu        sync.Mutex
	recoveryDepth     int
	replanDepth       int

	preserveWorktree       bool
	cycleCompletedNormally bool
	reachedPhaseEnd        bool
}

type dispatchResult struct {
	resp          PhaseResponse
	attemptCount  int
	phaseWorktree string
	treeGuard     *treediff.Guard
	beforeDirty   []string
	runner        PhaseRunner
	phaseReq      PhaseRequest
	remediation   bool
}

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

func (o *Orchestrator) recordFloorVerdictFailure(ctx context.Context, req CycleRequest, cycle int, failed Phase, state *State, cs *CycleState, diags []Diagnostic) {
	persistFloorFailReasons(cs, failed, diags)
	o.recordFailedApproachState(failureLearningRequest{
		CycleRequest: req,
		Cycle:        cycle,
		Failed:       failed,
		Err:          floorVerdictError(failed, diags),
		State:        state,
		CycleState:   cs,
	})
	o.writeFailureLearningState(ctx, state)
}

func floorVerdictError(phase Phase, diags []Diagnostic) error {
	return fmt.Errorf("%s %s", phase, verdictReason(VerdictFAIL, diags))
}

func verdictReason(verdict string, diags []Diagnostic) string {
	msgs := cyclestate.ErrorMessages(diags)
	if len(msgs) == 0 {
		return "verdict=" + verdict
	}
	return "verdict=" + verdict + ": " + strings.Join(msgs, "; ")
}

func (cr *cycleRun) recordChokepointEscape(reason string) {
	cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath,
		phaseOutcomeFrom(cr.current, PhaseResponse{Phase: string(cr.current)}, 0, reason, ""))
	cr.recordFailureLearning(cr.current, errors.New(reason), 0)
	cr.result.FinalVerdict = VerdictFAIL
}

func (o *Orchestrator) finalizeCycle(ctx context.Context, cs CycleState, cycle int, preCycleHEAD, projectRoot string, result *CycleResult, state *State, timings []phaseTimingEntry) (preserveWorktree bool, err error) {
	result.FinalVerdict = o.finalizeOutcome(result.FinalVerdict, result.RetroDecision, cs.Shipped)
	backfillFailReasons(result, timings)

	recordCrossArtifactInvariants(cycle, cs.WorkspacePath, cs.ActiveWorktree)

	if result.SystemFailure == nil {
		sig, reconciled := o.detectVerdictIncoherence(ctx, cs, result.FinalVerdict)
		switch {
		case reconciled:
			fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d verdict-coherence SELF-HEAL: recorded %s but on-disk audit=PASS, acs=PASS, and the audit-report fully verifies (challenge-token + sections + ADR-0039) — a benign clean-exit-late-write race; reconciled recorded verdict to PASS, not halting (ADR-0072).\n", cycle, result.FinalVerdict)
			result.FinalVerdict = VerdictPASS
		case sig != nil:
			result.SystemFailure = sig
		}
	}

	if result.SystemFailure == nil {
		if sig := detectLostLanding(landingEvidence{
			Workspace:    cs.WorkspacePath,
			ProjectRoot:  projectRoot,
			PreCycleHEAD: preCycleHEAD,
			Cycle:        cycle,
			Shipped:      cs.Shipped,
			FinalVerdict: result.FinalVerdict,
		}); sig != nil {
			result.SystemFailure = sig
			result.FinalVerdict = lostLandingVerdict()
		}
	}

	if shouldWarnSkippedUnknown(*result) {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d ended without shipping (%s): phases ran but this cycle's ship never landed and no audit-advisory block was recorded — any worktree changes were discarded. Inspect %s (audit-report.md verdict + acs-verdict.json red_count).\n", cycle, CycleOutcomeSkippedUnknown, cs.WorkspacePath)
	}

	postCycleHEAD, _ := o.gitHEAD()
	if o.throughputRecorder != nil && !hasThroughputCycle(state.TriageThroughput, cycle) && shippedOutcome(result.FinalVerdict, preCycleHEAD, postCycleHEAD) {
		o.throughputRecorder(state, cycle, cs.WorkspacePath)
	}

	preserveWorktree = preserveOnVerdict(result.FinalVerdict)

	if preserveWorktree {
		o.stampContinuationManifest(ctx, cs, cycle, projectRoot)
	}

	o.carryover().Closeout(state, cs.WorkspacePath, cycle, time.Now().UTC())

	state.LastCycleNumber = max(state.LastCycleNumber, cycle)
	if perr := o.persistCycleEndState(ctx, *state); perr != nil {
		return preserveWorktree, fmt.Errorf("write state: %w", perr)
	}
	return preserveWorktree, nil
}

type cycleInit struct {
	state         State
	cs            CycleState
	cycle         int
	consoleLeased map[string]bool
}

func (o *Orchestrator) newCycleRun(ctx context.Context, req CycleRequest) (cycleInit, func(preserve, completedNormally bool), error) {
	var stack []func(preserve, completedNormally bool)
	run := func(preserve, completedNormally bool) {
		for i := len(stack) - 1; i >= 0; i-- {
			stack[i](preserve, completedNormally)
		}
	}
	failClean := func() { run(false, false) }

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
	cycle, err := o.allocateCycle(ctx, &state, req.ProjectRoot)
	if err != nil {
		failClean()
		return cycleInit{}, nil, fmt.Errorf("allocate cycle: %w", err)
	}

	startedAt := o.now().UTC().Format(time.RFC3339)
	intentRequired := req.Context["intent_required"] == "true" ||
		o.workflowConfig.PhaseEnables["intent"] == "on"
	runID := MintRunID(o.now())
	o.currentRunID.Store(runID)
	stack = append(stack, func(_, _ bool) { o.currentRunID.Store("") })
	cs := CycleState{
		CycleID:                         cycle,
		GoalHash:                        req.GoalHash,
		GoalText:                        req.Context["goal"],
		Phase:                           string(PhaseStart),
		StartedAt:                       startedAt,
		PhaseStartedAt:                  startedAt,
		WorkspacePath:                   RunWorkspacePath(req.ProjectRoot, cycle),
		IntentRequired:                  intentRequired,
		RunID:                           runID,
		ExplanationDocumentationVersion: o.explanationContractVersion,
	}
	if os.Getenv(ipcenv.FleetKey) != "" && cs.WorkspacePath != "" {
		if err := fleetIsolationSetenv(ipcenv.CycleStateFileKey, filepath.Join(cs.WorkspacePath, CycleStateFile)); err != nil {
			failClean()
			return cycleInit{}, nil, fmt.Errorf("fleet cycle-state isolation: set %s: %w", ipcenv.CycleStateFileKey, err)
		}
		stack = append(stack, func(_, _ bool) { _ = os.Unsetenv(ipcenv.CycleStateFileKey) })
	}
	guardDisabled := req.DisableWorkspaceGuard
	if !guardDisabled {
		if err := archivePollutedWorkspace(cs.WorkspacePath, o.now); err != nil {
			fmt.Fprintf(o.errOut(), "[orchestrator] WARN workspace archive failed: %v\n", err)
		}
	}
	consoleLeased := adoptConsoleLease(req.ProjectRoot, time.Now(), o.errOut())
	wtPath, werr := o.worktree.Create(req.ProjectRoot, cycle)
	if werr != nil && fleetMode(req.Env) {
		if wtPath, werr = o.reprovisionWorktree(req.ProjectRoot, cycle, werr); werr != nil {
			failClean()
			return cycleInit{}, nil, o.deferLaneWithoutWorktree(cs, werr)
		}
	}
	if werr != nil {
		o.recordProvisioningFailure(cycle, req.ProjectRoot, cs.WorkspacePath, werr)
	} else if sourceErr := verifyFreshCycleSource(wtPath, cycle); sourceErr != nil {
		failClean()
		return cycleInit{}, nil, fmt.Errorf("verify fresh cycle %d source in preserved worktree %s: %w", cycle, wtPath, sourceErr)
	} else {
		cs.ActiveWorktree = wtPath
		if inPlaceWorktree(wtPath, req.ProjectRoot) {
			fmt.Fprintf(o.errOut(), "[orchestrator] WARN cycle %d: the active worktree is the project root %s (in-place provisioner) — host normalization, leak recovery, salvage snapshot and fleet rebase are disabled: a cycle never mutates the operator's tree\n", cycle, wtPath)
		}
		stack = append(stack, func(preserve, completedNormally bool) {
			o.teardownCycleWorktree(req.ProjectRoot, wtPath, preserve, completedNormally)
		})
		base, berr := resolveWorktreeBaseSHA(ctx, wtPath)
		if berr != nil {
			fmt.Fprintf(o.errOut(), "[orchestrator] WARN worktree-normalize: resolving the worktree base at creation failed: %v (build-commit normalize disabled this cycle)\n", berr)
			if errors.Is(berr, errUnresolvableSnapshotBase) {
				failClean()
				return cycleInit{}, nil, fmt.Errorf("resolve worktree base: %w", berr)
			}
		} else {
			cs.WorktreeBaseSHA = strings.TrimSpace(base)
		}
	}
	if err := o.storage.WriteCycleState(ctx, cs); err != nil {
		failClean()
		return cycleInit{}, nil, fmt.Errorf("init cycle-state: %w", err)
	}

	stopLease := startRunLease(cs.WorkspacePath, runID, o.now, leaseRefreshInterval())
	stack = append(stack, func(_, _ bool) { stopLease() })

	return cycleInit{
		state:         state,
		cs:            cs,
		cycle:         cycle,
		consoleLeased: consoleLeased,
	}, run, nil
}

func (o *Orchestrator) clearActiveWorktree(wtPath string) {
	ctx := context.Background()
	cs, err := o.storage.ReadCycleState(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN worktree teardown: cycle-state read failed, pruned path %s left named: %v\n", wtPath, err)
		return
	}
	if cs.ActiveWorktree != wtPath {
		return
	}
	cs.ActiveWorktree = ""
	if err := o.storage.WriteCycleState(ctx, cs); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN worktree teardown: cycle-state write failed, pruned path %s left named: %v\n", wtPath, err)
	}
}

type cyclePlan struct {
	envSnap       map[string]string
	ctxSnap       map[string]string
	preCycleHEAD  string
	benchedCLIs   []router.BenchedCLI
	clampedPlan   *router.PhasePlan
	directivesSet directives.Set
}

func (o *Orchestrator) advisorPlanInput(ctx context.Context, current string, signals router.RoutingSignals, req CycleRequest, state State, cs CycleState, cycle int, env map[string]string, benchedCLIs []router.BenchedCLI) router.RouteInput {
	lastReason, lessons := o.recallForPlan(ctx, state.FailedAt, req.Context["goal"])
	unavailable := o.unavailableOptionalPhases()
	return router.RouteInput{
		Current:           current,
		Signals:           signals,
		Cfg:               o.cfg,
		Now:               o.now(),
		Workspace:         cs.WorkspacePath,
		ProjectRoot:       req.ProjectRoot,
		ActiveWorktree:    cs.ActiveWorktree,
		Cycle:             cycle,
		Env:               env,
		LastReason:        lastReason,
		Lessons:           lessons,
		Catalog:           withoutCards(phaseCardsFromCatalog(o.catalog), unavailable),
		OnDemandPhases:    withoutNames(onDemandCatalogNames(o.catalog), unavailable),
		GoalText:          req.Context["goal"],
		LaneItems:         o.laneItemsForAdvisor(req.ProjectRoot, cs.WorkspacePath),
		CarryoverTodos:    carryoverTodosForAdvisor(state.CarryoverTodos),
		BenchedCLIs:       benchedCLIs,
		UnavailablePhases: unavailable,
		IntentRequired:    cs.IntentRequired,
		PSMASEnabled:      o.workflowConfig.PSMASEnabled,
	}
}

func (o *Orchestrator) planCycle(ctx context.Context, req CycleRequest, state State, cs CycleState, cycle int) cyclePlan {
	if o.catalogRefresh != nil {
		action := "ok"
		if err := o.catalogRefresh(ctx); err != nil {
			fmt.Fprintf(o.errOut(), "[orchestrator] WARN model-catalog refresh failed: %v\n", err)
			action = "failed"
		}
		stage := ""
		if o.catalogRefreshStage != nil {
			stage = o.catalogRefreshStage()
		}
		if err := o.ledger.Append(ctx, LedgerEntry{
			TS:      o.now().UTC().Format(time.RFC3339),
			Cycle:   cycle,
			Role:    "orchestrator",
			Kind:    "catalog_refresh",
			Action:  action,
			Message: stage,
		}); err != nil {
			fmt.Fprintf(o.errOut(), "[orchestrator] WARN catalog-refresh ledger stamp failed: %v\n", err)
		}
	}

	var directivesSet directives.Set
	if o.directivesProvider != nil {
		directivesSet = o.directivesProvider(ctx, cycle)
		if directivesSet.Version != "" {
			if err := o.ledger.Append(ctx, LedgerEntry{
				TS:     o.now().UTC().Format(time.RFC3339),
				Cycle:  cycle,
				Role:   "orchestrator",
				Kind:   "operator_directives",
				Action: directivesSet.Version,
			}); err != nil {
				fmt.Fprintf(o.errOut(), "[orchestrator] WARN operator-directives ledger stamp failed: %v\n", err)
			}
		}
	}

	envSnap := make(map[string]string, len(req.Env))
	for k, v := range req.Env {
		envSnap[k] = v
	}
	ctxSnap := make(map[string]string, len(req.Context))
	for k, v := range req.Context {
		ctxSnap[k] = v
	}

	if ls := loadLaneScope(cs.WorkspacePath); ls != nil {
		ctxSnap["fleet_scope"] = strings.Join(ls.TodoIDs, ",")
	} else if scope := envSnap[ipcenv.FleetScopeKey]; scope != "" {
		ctxSnap["fleet_scope"] = scope
		materializeLaneScope(cs.WorkspacePath, scope, req.GoalHash)
	}
	if scope := ctxSnap["fleet_scope"]; scope != "" && o.scopePathFor != nil {
		var pairs []string
		for _, id := range strings.Split(scope, ",") {
			if id = strings.TrimSpace(id); id == "" {
				continue
			}
			if p := o.scopePathFor(req.ProjectRoot, id); p != "" {
				if strings.ContainsAny(id, "= \t") || strings.ContainsAny(p, " \t\n") {
					fmt.Fprintf(o.errOut(), "[orchestrator] WARN fleet_scope_paths: cannot encode id %q path %q (id must not contain '=' or blanks; path must not contain blanks) — left unresolved\n", id, p)
					continue
				}
				pairs = append(pairs, id+"="+p)
			}
		}
		if len(pairs) > 0 {
			ctxSnap["fleet_scope_paths"] = strings.Join(pairs, " ")
		}
	}

	if _, alreadySet := ctxSnap["challengeToken"]; !alreadySet {
		var tokBytes [8]byte
		if _, err := challengeTokenRead(tokBytes[:]); err == nil {
			tok := hex.EncodeToString(tokBytes[:])
			ctxSnap["challengeToken"] = tok
			_ = os.MkdirAll(cs.WorkspacePath, 0o755)
			if err := os.WriteFile(filepath.Join(cs.WorkspacePath, "challenge-token.txt"), []byte(tok+"\n"), 0o644); err != nil {
				fmt.Fprintf(o.errOut(), "[orchestrator] WARN challenge-token.txt write failed: %v (Context route still works)\n", err)
			}
		} else {
			fmt.Fprintf(o.errOut(), "[orchestrator] WARN challenge token mint failed: %v (phase agents will fall back to their own protocol)\n", err)
		}
	}

	seedChronicleDigest(req.ProjectRoot, cs, state, o.chronicle, ctxSnap, o.errOut())

	preCycleHEAD, _ := o.gitHEAD()

	benchedCLIs := benchedCLIsForRouting(req.ProjectRoot)
	var clampedPlan *router.PhasePlan
	if o.cfg.Stage >= config.StageAdvisory && o.cfg.Mode == config.ModeDynamicLLM && o.planner != nil {
		planIn := o.advisorPlanInput(ctx, string(PhaseStart), router.RoutingSignals{}, req, state, cs, cycle, envSnap, benchedCLIs)
		if raw, perr := o.observedPlan(ctx, cs, cycle, planIn); perr != nil {
			fmt.Fprintf(o.errOut(), "[orchestrator] WARN phase advisor Plan failed (degrading to static spine): %v\n", perr)
		} else if raw != nil {
			o.recordPlanRejections(ctx, cycle, cs, router.ValidatePlan(planIn, raw))
			var clamps []router.Clamp
			clampedPlan, clamps = router.ClampPlanToFloorWith(planIn, raw, o.resolvedShipFloor(), cs.IntentRequired)
			if o.cfg.ModelRouting != config.ModelRoutingStatic {
				var mrClamps []router.Clamp
				profileFor := func(phase string) *profiles.Profile {
					return o.profileForModelRouting(req.ProjectRoot, phase)
				}
				clampedPlan, mrClamps = router.ClampPlanModelRouting(clampedPlan, profileFor, o.modelCatalogLookup)
				clamps = append(clamps, mrClamps...)
			}
			o.recordPhasePlan(ctx, cycle, cs, clampedPlan, clamps)
			o.registerMintedPhases(clampedPlan)
		}
	}

	return cyclePlan{
		envSnap:       envSnap,
		ctxSnap:       ctxSnap,
		preCycleHEAD:  preCycleHEAD,
		benchedCLIs:   benchedCLIs,
		clampedPlan:   clampedPlan,
		directivesSet: directivesSet,
	}
}

func (o *Orchestrator) profileForModelRouting(projectRoot, phase string) *profiles.Profile {
	prof, _ := phaseProfile(projectRoot, phase)
	return prof
}
