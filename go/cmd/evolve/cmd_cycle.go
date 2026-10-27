package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/rootwire"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/mintregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseregistrar"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/build"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/buildplanner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/debugger"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/intent"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/scout"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/specrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/swarmrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/tdd"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/triage"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/research"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/topngate"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

func runCycle(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve cycle: missing subcommand (try: run | reset | timing | models)")
		return 10
	}
	switch args[0] {
	case "run":
		return runCycleRun(args[1:], stdout, stderr)
	case "reset":
		return runCycleReset(args[1:], stdout, stderr)
	case "timing":
		return runCycleTiming(args[1:], stdout, stderr)
	case "models":
		return runCycleModels(args[1:], stdout, stderr)
	case "outputs":
		return runCycleOutputs(args[1:], stdout, stderr)
	case "scope-delta":
		return runCycleScopeDelta(args[1:], stdout, stderr)
	case "reaudit":
		return runCycleReaudit(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve cycle: unknown subcommand %q\n", args[0])
		return 10
	}
}

func cycleResetRootWarner(stderr io.Writer) func(string) {
	return func(m string) {
		fmt.Fprintf(stderr, "evolve cycle reset: WARN: %s\n", m)
	}
}

type cycleResetResult struct {
	Cycle     int    `json:"cycle"`
	Phase     string `json:"phase"`
	Archive   string `json:"archive"`
	NextCycle int    `json:"next_cycle"`
	DryRun    bool   `json:"dry_run"`
	Forced    bool   `json:"forced"`
}

func runCycleReset(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve cycle reset", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		projectRoot string
		evolveDir   string
		reason      string
		dryRun      bool
		force       bool
		asJSON      bool
	)
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to the project root (default cwd)")
	fs.StringVar(&evolveDir, "evolve-dir", "", "path to .evolve/ state directory (default <project-root>/.evolve)")
	fs.StringVar(&reason, "reason", "operator-requested reset", "reason recorded in the seal manifest + ledger")
	fs.BoolVar(&dryRun, "dry-run", false, "print the seal plan without mutating any state")
	fs.BoolVar(&force, "force", false, "seal even when the cycle's run lease is still fresh (override a live, heartbeating owner — last resort)")
	fs.BoolVar(&asJSON, "json", false, "print the result as JSON")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	projectRoot = paths.AbsoluteRoot("--project-root", projectRoot, cycleResetRootWarner(stderr))
	if evolveDir == "" {
		evolveDir = filepath.Join(projectRoot, ".evolve")
	}

	res, err := core.SealCycle(context.Background(), ledger.New(evolveDir), core.SealOptions{
		EvolveDir:   evolveDir,
		ProjectRoot: projectRoot,
		Reason:      reason,
		DryRun:      dryRun,
		Force:       force,
		PidAlive:    pidAlive,
	})
	if err != nil {
		return cycleResetFailure(res, err, stderr)
	}
	if res.ForcedOverLiveOwner {
		fmt.Fprintf(stderr, "evolve cycle reset: WARN --force sealed cycle %d while its lease was still FRESH (pid %d, heartbeat %s ago) — a live owner was overridden.\n",
			res.SealedCycleID, res.LeaseOwnerPID, res.LeaseHeartbeatAge.Round(time.Second))
	}
	if asJSON {
		return printVerbJSON("cycle reset", cycleResetResult{Cycle: res.SealedCycleID, Phase: res.SealedPhase, Archive: res.ArchiveDir,
			NextCycle: res.NextCycle, DryRun: res.DryRun, Forced: res.ForcedOverLiveOwner}, stdout, stderr)
	}
	verb := "sealed"
	if res.DryRun {
		verb = "would seal"
	}
	fmt.Fprintf(stdout, "%s cycle %d (phase=%s) → %s; next cycle %d\n",
		verb, res.SealedCycleID, res.SealedPhase, res.ArchiveDir, res.NextCycle)
	return 0
}

func cycleResetFailure(res core.SealResult, err error, stderr io.Writer) int {
	switch {
	case errors.Is(err, core.ErrNothingToReset):
		fmt.Fprintln(stderr, "evolve cycle reset: no in-progress cycle to seal")
		return 1
	case errors.Is(err, core.ErrCycleOwnedLive):
		fmt.Fprintf(stderr, "evolve cycle reset: cycle %d is owned by a LIVE run (pid %d, lease heartbeat %s ago) — refusing to seal a running loop.\n",
			res.SealedCycleID, res.LeaseOwnerPID, res.LeaseHeartbeatAge.Round(time.Second))
		fmt.Fprintln(stderr, "  • continue it:      evolve loop --resume")
		fmt.Fprintln(stderr, "  • stop it cleanly:  send SIGINT/SIGTERM (Ctrl-C) to that loop — it checkpoints, then `evolve loop --resume`")
		fmt.Fprintln(stderr, "  • override (only if you KNOW the owner is wedged): evolve cycle reset --force")
		fmt.Fprintln(stderr, "  Do NOT `pkill evolve` or `tmux kill-server` — that corrupts the run and sibling sessions.")
		return cliExitRefused
	default:
		fmt.Fprintf(stderr, "evolve cycle reset: %v\n", err)
		return 1
	}
}

var cycleRunAbsoluteRoot = paths.AbsoluteRoot

type cycleRunFlags struct {
	projectRoot, goalHash, goalText, evolveDir string
	simulate, bypassPolicy                     bool
}

func parseCycleRunFlags(args []string, stderr io.Writer) (cycleRunFlags, bool) {
	var f cycleRunFlags
	fs := flag.NewFlagSet("evolve cycle run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&f.projectRoot, "project-root", ".", "absolute path to the project root (default cwd)")
	fs.StringVar(&f.goalHash, "goal-hash", "", "8-char SHA256 of the goal (required)")
	fs.StringVar(&f.goalText, "goal", "", "human-readable goal text (threaded to Scout + the routing advisor as Context[\"goal\"]); optional")
	fs.StringVar(&f.evolveDir, "evolve-dir", "", "path to .evolve/ state directory (default <project-root>/.evolve)")
	fs.BoolVar(&f.simulate, "simulate", false, "no-LLM walk: every phase returns PASS without calling out (for parity-audit harness)")
	fs.BoolVar(&f.bypassPolicy, "bypass-policy", false, "use --bypass-policy to bypass policy.json pin enforcement for every phase this run (operator escape hatch)")
	args = stripRemovedBudgetFlags(args, func(m string) {
		fmt.Fprintf(stderr, "evolve cycle run: WARN: %s\n", m)
	})
	if err := fs.Parse(args); err != nil {
		return f, false
	}
	if f.goalHash == "" {
		fmt.Fprintln(stderr, "evolve cycle run: --goal-hash is required")
		return f, false
	}
	return f, true
}

func (f cycleRunFlags) request(projectRoot string, environ []string) core.CycleRequest {
	return core.CycleRequest{
		ProjectRoot:           projectRoot,
		GoalHash:              f.goalHash,
		Env:                   carriedEnv(environ),
		Context:               cycleContext(f.goalHash, f.goalText),
		DisableWorkspaceGuard: disableWorkspaceGuardForTest,
		BypassPolicy:          f.bypassPolicy,
	}
}

func runCycleRun(args []string, stdout, stderr io.Writer) int {
	f, ok := parseCycleRunFlags(args, stderr)
	if !ok {
		return 10
	}
	projectRoot, evolveDir := f.projectRoot, f.evolveDir
	projectRoot = cycleRunAbsoluteRoot("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve cycle run: WARN: %s\n", m)
	})
	if evolveDir == "" {
		evolveDir = filepath.Join(projectRoot, ".evolve")
	}
	if err := fleetLaneEvolveDirOK(projectRoot, evolveDir); err != nil {
		fmt.Fprintf(stderr, "evolve cycle run: %v\n", err)
		return 10
	}

	d := wireCycleRun(f, projectRoot, evolveDir, stderr)
	defer d.closeSignals()
	if d.RoutingErr != nil {
		fmt.Fprintf(stderr, "evolve cycle run: %v\n", d.RoutingErr)
		return exitRoutingRefused
	}
	var lifecycleLedger inboxmover.LedgerAppender = d.Ledger
	orch, signals := d.Orchestrator, d.Signals
	result, err := orch.RunCycle(context.Background(), f.request(projectRoot, os.Environ()))
	if errors.Is(err, core.ErrAllFamiliesExhausted) {
		return cycleRunQuotaExit(result.Cycle, projectRoot, evolveDir, fleetLaneFromEnv(), stdout, stderr, lifecycleLedger, signals)
	}
	if err != nil {
		return cycleRunErrorExit(err, result.Cycle, projectRoot, evolveDir, stderr, lifecycleLedger, signals)
	}
	buf, _ := json.MarshalIndent(result, "", "  ")
	fmt.Fprintln(stdout, string(buf))
	if sf := result.SystemFailure; sf != nil && sf.Halt {
		return haltOnSystemFailure(evolveDir, projectRoot, result.Cycle, cycleWorkspace(projectRoot, result.Cycle), sf, stderr, signals, systemFailureRule)
	}
	if applied, err := closeoutCycleOutcome(result, projectRoot, evolveDir, stderr, lifecycleLedger, signals); err != nil {
		fmt.Fprintf(stderr, "evolve cycle run: WARN: could not apply cycle %d %s to the inbox: %v\n", result.Cycle, applied, err)
	}
	return cycleRunExitCode(result)
}

func cycleRunErrorExit(err error, cycle int, projectRoot, evolveDir string, stderr io.Writer, lifecycle inboxmover.LedgerAppender, signals *signalcenter.Center) int {
	var deferral *core.LaneDeferral
	if errors.As(err, &deferral) {
		if rerr := releaseDeferredLaneClaims(projectRoot, deferral.Cycle, cyclestate.CycleTerminationLaneWorktreeDeferred, stderr, lifecycle, signals); rerr != nil {
			fmt.Fprintf(stderr, "evolve cycle run: WARN: could not release cycle %d claims: %v\n", deferral.Cycle, rerr)
		}
		fmt.Fprintf(stderr, "evolve cycle run: %v\n", err)
		return fleet.ExitDeferred
	}
	warnCycleFailureOutcome(stderr, cycle, applyCycleFailureOutcome(projectRoot, evolveDir, cycle, stderr, lifecycle, signals))
	fmt.Fprintf(stderr, "evolve cycle run: %v\n", err)
	return 1
}

func releaseDeferredLaneClaims(projectRoot string, cycle int, reason string, stderr io.Writer, lifecycle inboxmover.LedgerAppender, signals *signalcenter.Center) error {
	opts := inboxmover.Options{ProjectRoot: projectRoot, Ledger: lifecycle, Stderr: stderr, Signals: signals}
	_, err := inboxmover.ReleaseCycleProcessingWithReason(opts, cycle, reason)
	return err
}

var protectedMembership = guards.IsProtectedSurface

func closeoutCycleOutcome(result core.CycleResult, projectRoot, evolveDir string, stderr io.Writer, lifecycle inboxmover.LedgerAppender, signals *signalcenter.Center) (applied string, err error) {
	switch {
	case result.FinalVerdict == cyclestate.VerdictFAIL:
		return "failure outcome", applyResultFailureOutcome(result, projectRoot, evolveDir, stderr, lifecycle, signals)
	case core.IsTriageNoWorkResult(result):
		return "no-work hand-off", applyCycleNoWorkOutcome(projectRoot, result.Cycle, stderr, lifecycle, signals)
	}
	return "", nil
}

func applyCycleNoWorkOutcome(projectRoot string, cycle int, stderr io.Writer, lifecycle inboxmover.LedgerAppender, signals *signalcenter.Center) error {
	res, err := cycleoutcome.ApplyNoWork(cycleoutcome.NoWorkInputs{
		ProjectRoot: projectRoot,
		Workspace:   cycleWorkspace(projectRoot, cycle),
		Cycle:       cycle,
		Stderr:      stderr,
	}.WithLedger(lifecycle).WithSignals(signals))
	if len(res.Routed)+len(res.Retired) > 0 {
		fmt.Fprintf(stderr, "[cycle] planned no-work closeout for cycle %d: routed %v to the console, retired %v (no inbox item)\n", cycle, res.Routed, res.Retired)
	}
	return err
}

func applyCycleFailureOutcome(projectRoot, evolveDir string, cycle int, stderr io.Writer, lifecycle inboxmover.LedgerAppender, signals *signalcenter.Center) error {
	return applyResultFailureOutcome(core.CycleResult{Cycle: cycle}, projectRoot, evolveDir, stderr, lifecycle, signals)
}

func applyResultFailureOutcome(result core.CycleResult, projectRoot, evolveDir string, stderr io.Writer, lifecycle inboxmover.LedgerAppender, signals *signalcenter.Center) error {
	if result.Cycle == 0 {
		return nil
	}
	in := cycleoutcome.FailureInputsFor(projectRoot, evolveDir, cycleWorkspace(projectRoot, result.Cycle), result.Cycle, stderr)
	in.Protected = protectedMembership
	in.Defer, _ = core.DeferOf(result)
	_, err := cycleoutcome.ApplyFailure(in.WithLedger(lifecycle).WithSignals(signals))
	return err
}

func warnCycleFailureOutcome(stderr io.Writer, cycle int, err error) {
	if err != nil {
		fmt.Fprintf(stderr, "evolve cycle run: WARN: could not apply cycle %d failure outcome to the inbox: %v\n", cycle, err)
	}
}

func wireCycleRun(f cycleRunFlags, projectRoot, evolveDir string, stderr io.Writer) orchDeps {
	if f.simulate {
		return wireSimulateOrchestrator(projectRoot, evolveDir, stderr)
	}
	gcOrphanSessions("cycle-start", stderr)
	return wireOrchestratorDepsFn(projectRoot, evolveDir, stderr, routingRun{bypass: f.bypassPolicy, env: filterEvolveEnv(os.Environ())})
}

func filterEvolveEnv(environ []string) map[string]string {
	return cmdutil.FilterEvolveEnv(environ)
}

const (
	roleCycle     = "cycle"
	roleLoop      = "loop"
	roleLoopChain = "loop-chain"
	roleShip      = "ship"
	roleSubagent  = "subagent"
	roleSimulate  = "simulate"
	roleDispatch  = "dispatch"
	roleAgent     = "agent"
)

func agentRunSignals(projectRoot string, console io.Writer) (*signalcenter.Center, func()) {
	return newRoleSignalCenter(roleAgent, projectRoot, filepath.Join(projectRoot, ".evolve"), console)
}

func agentRunBridge(projectRoot string, signals *signalcenter.Center) core.Bridge {
	return bridge.NewDefault(projectRoot, signals)
}

type orchDeps struct {
	Storage      core.Storage
	Ledger       rootLedger
	Orchestrator *core.Orchestrator
	Signals      *signalcenter.Center
	CloseSignals func()
	Bridge       *bridge.Adapter
	Runners      map[core.Phase]core.PhaseRunner
	Router       *cliroute.Router
	RoutingErr   error
}

func (d orchDeps) closeSignals() {
	if d.CloseSignals == nil {
		d.Signals.Flush()
		return
	}
	d.CloseSignals()
}

type rootLedger interface {
	core.Ledger
	inboxmover.LedgerAppender
}

func newRootSignalCenter(projectRoot, evolveDir string, console io.Writer) *signalcenter.Center {
	signals := signalcenter.New(signalcenter.WithPID(os.Getpid()))
	signals.Subscribe(listenWhen(cycleStamped, signals.NDJSONSink(func(cycle int) string {
		return filepath.Join(core.RunWorkspacePath(projectRoot, cycle), signalcenter.StreamFileName)
	})))
	signals.Subscribe(listenWhen(legacyProjectRecord, signals.NDJSONSink(func(int) string {
		return filepath.Join(evolveDir, signalcenter.StreamFileName)
	})))
	signals.Subscribe(signalcenter.ConsoleSink(console))
	return signals
}

func listenWhen(keep func(signalcenter.Event) bool, l signalcenter.Listener) signalcenter.Listener {
	return func(e signalcenter.Event) {
		if keep(e) {
			l(e)
		}
	}
}

func cycleStamped(e signalcenter.Event) bool { return e.Cycle != 0 }

var legacyProjectKinds = map[signalcenter.Kind]bool{
	signalcenter.KindPhaseDispatched: true, signalcenter.KindPhaseOutcome: true, signalcenter.KindPhaseAborted: true,
	signalcenter.KindGateRejected: true, signalcenter.KindGateCorrected: true, signalcenter.KindGatePassed: true,
	signalcenter.KindShipLanded: true, signalcenter.KindShipError: true, signalcenter.KindShipWarning: true,
	signalcenter.KindSystemFailure: true, signalcenter.KindQuotaPaused: true, signalcenter.KindBridgeWarning: true,
	signalcenter.KindBridgeTripwire: true, signalcenter.KindPaneLiveness: true, signalcenter.KindLedgerAppended: true,
	signalcenter.KindOutcomeWarning: true, signalcenter.KindFailureDiagWarning: true, signalcenter.KindCarryoverWarning: true,
	signalcenter.KindFailureLearningWarning: true, signalcenter.KindConfigWarning: true, signalcenter.KindObserverWarning: true,
	signalcenter.KindAuditWarning: true, signalcenter.KindInboxWarning: true, signalcenter.KindRunnerWarning: true,
	signalcenter.KindAdvisorWarning: true, signalcenter.KindCycleSealed: true, signalcenter.KindLoopWave: true,
	signalcenter.KindLoopWarning: true, signalcenter.KindLoopHalt: true, signalcenter.KindLoopEscalation: true,
	signalcenter.KindListenerPanicked: true, signalcenter.KindRegistryDrift: true, signalcenter.KindSinkDropped: true,
}

func legacyProjectRecord(e signalcenter.Event) bool {
	return e.Cycle == 0 && legacyProjectKinds[e.Kind]
}

var attachRootEventsFn = rootwire.Attach

func newRoleSignalCenter(role, projectRoot, evolveDir string, console io.Writer) (*signalcenter.Center, func()) {
	signals := newRootSignalCenter(projectRoot, evolveDir, console)
	return signals, attachRootEventsFn(signals, role, evolveDir, console)
}

func wireOrchestratorDeps(projectRoot, evolveDir string, console io.Writer, run routingRun) orchDeps {
	if evolveDir != "" {
		d := evolveDir
		bridge.SetModelCatalogDirFn(func() string { return d })
	}
	signals, closeSignals := newRoleSignalCenter(run.centerRole(), projectRoot, evolveDir, console)
	watchOptimisticAdmission(signals, os.Getenv)
	st := storage.New(evolveDir)
	ld := ledger.New(evolveDir, ledger.WithSignals(signals))

	br := bridge.NewDefault(projectRoot, signals)
	br.SetOnStopReview(stopReviewRecorder(ld))
	prm := cmdutil.NewPromptsLoader(projectRoot)

	registryPath := config.RegistryPath(projectRoot)
	loader := wiredRoutingConfigLoader(signals)
	cfg, _ := loader.Load(registryPath, filterEvolveEnv(os.Environ()))

	var policyShipFloor []string
	pol, policyErr := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if policyErr != nil {
		fmt.Fprintf(console, "[policy] WARN %v (mandatory merge skipped; the routing table refuses it before any dispatch)\n", policyErr)
		pol = policy.Policy{}
	} else {
		cfg.Mandatory = pol.MergeMandatory(cfg.Mandatory)
		if floor, overridden := pol.FloorPhases(); overridden {
			policyShipFloor = floor
		}
		cfg.AuditFailRoutesTo = router.FailureRouteFromPolicy(pol)
	}
	swCfg := swarmrunner.Config{Stage: pol.SwarmConfig().Stage, PortBase: pol.SwarmConfig().PortBase, WorktreeBase: pol.WorktreeBase()}
	gatesCfg := pol.GatesConfig()
	routerCfg := pol.RouterConfig()
	recoveryCfg := pol.RecoveryConfig()
	cfg, _ = loader.ApplyPolicyStages(cfg, policyStagesOf(gatesCfg, recoveryCfg, routerCfg, pol.ParallelEvaluateConfig()))
	wfCfg := pol.WorkflowConfig()

	catalog, builtinCat, userSpecs := loadPhaseCatalog(projectRoot, registryPath, prm, signals.Emit)
	cliRouter, routingErr := wireCLIRouter(routerSite{root: projectRoot, catalog: catalog}, run, console)
	if routingErr != nil {
		return orchDeps{Signals: signals, CloseSignals: closeSignals, RoutingErr: routingErr}
	}
	runner.DefaultRouter = cliRouter

	explainUsage := usageEvidenceFn(projectRoot, evolveDir, console)
	walked := bridgechain.New(withUsageEvidence(br, explainUsage, signals),
		bridgechain.DefaultPlanResolver(cliRouter),
		bridgechain.WithBench(cycleBenchOnEscalation),
		bridgechain.WithLog(cycleDiagf))

	var contractVerifier runner.ContractVerifier = deliverable.PlainVerifier{PhaseIO: cfg.PhaseIO}
	verifierOf := func() runner.ContractVerifier { return contractVerifier }
	var hostEffects core.HostEffects
	hostEffectsOf := func() core.HostEffects { return hostEffects }
	forbidden := laneForbiddenTo(projectRoot, signals.Emit)

	runners := map[core.Phase]core.PhaseRunner{
		core.PhaseIntent:       intent.New(intent.Config{Bridge: walked, Prompts: prm, ContractVerifier: verifierOf, HostEffects: hostEffectsOf, CompactPrompts: cfg.CompactPrompts}),
		core.PhaseScout:        swarmrunner.New(scout.New(scout.Config{Bridge: walked, Prompts: prm, ContractVerifier: verifierOf, HostEffects: hostEffectsOf, PhaseIO: cfg.PhaseIO, CompactPrompts: cfg.CompactPrompts}), walked, swarm.ModeReader, swCfg),
		core.PhaseTriage:       triage.New(triage.Config{Bridge: walked, Prompts: prm, ContractVerifier: verifierOf, HostEffects: hostEffectsOf, PhaseIO: cfg.PhaseIO, CompactPrompts: cfg.CompactPrompts, LaneForbidden: forbidden}),
		core.PhaseTDD:          tdd.New(tdd.Config{Bridge: walked, Prompts: prm, ContractVerifier: verifierOf, HostEffects: hostEffectsOf, CompactPrompts: cfg.CompactPrompts}),
		core.PhaseBuildPlanner: buildplanner.New(buildplanner.Config{Bridge: walked, Prompts: prm, ContractVerifier: verifierOf, HostEffects: hostEffectsOf}).BaseRunner(),
		core.PhaseBuild:        swarmrunner.New(build.New(build.Config{Bridge: walked, Prompts: prm, ContractVerifier: verifierOf, HostEffects: hostEffectsOf, PhaseIO: cfg.PhaseIO, CompactPrompts: cfg.CompactPrompts}), walked, swarm.ModeWriter, swCfg),
		core.PhaseAudit:        audit.NewDefaultWithStageCompactSpec(walked, prm, cfg.PhaseIO, cfg.CompactPrompts, documentSpecPtr(cfg), audit.WithContractVerifier(verifierOf), audit.WithHostEffects(hostEffectsOf), audit.WithSignals(func() *signalcenter.Center { return signals })),
		core.PhaseShip:         ship.New(ship.Config{Runner: sysexec.DefaultRunner, PhaseIO: cfg.PhaseIO, ManifestGate: gatesCfg.ManifestGate, RepoContractGate: gatesCfg.RepoContractGate, Signals: signals}),
		core.PhaseRetro:        retro.New(retro.Config{Bridge: walked, Prompts: prm, Model: "auto", CompactPrompts: cfg.CompactPrompts, Router: cliRouter, ContractVerifier: verifierOf}),
		core.PhaseDebugger:     debugger.New(debugger.Config{Bridge: walked, Prompts: prm, ContractVerifier: verifierOf, HostEffects: hostEffectsOf, CompactPrompts: cfg.CompactPrompts}),
	}

	br.SetContractResolver(phasecontract.NewCatalogResolver(catalog.Get))
	wireBridgeStages(br, cfg)
	emitRoutingDefects(signals.Emit, "wireOrchestratorDeps", phasespec.ApplyUserRouting(&cfg, userSpecs, builtinCat))
	for _, s := range catalog.UserPhases() {
		if len(phasespec.ValidateUserSpecWithCatalog(s, builtinCat)) > 0 {
			continue
		}
		if _, exists := runners[core.Phase(s.Name)]; !exists {
			runners[core.Phase(s.Name)] = specrunner.New(s, specrunner.Config{Bridge: walked, Prompts: prm, ContractVerifier: verifierOf, HostEffects: hostEffectsOf})
		}
	}
	registerBuiltinSpecRunners(runners, builtinCat, specrunner.Config{Bridge: walked, Prompts: prm, ContractVerifier: verifierOf, HostEffects: hostEffectsOf}, os.Stderr)
	advisor, advErr := wireRouterAdvisor(routerAdvisorWiring{router: cliRouter, root: projectRoot, bridge: walked, prompts: prm, signals: signals})
	if advErr != nil {
		return orchDeps{Signals: signals, CloseSignals: closeSignals, RoutingErr: advErr}
	}
	strategy := router.Select(cfg, advisor)
	opts := []core.Option{
		core.WithRouting(cfg, strategy),
		core.WithCatalog(catalog),
		core.WithPlanner(advisor),
		core.WithFailureAdviser(core.NewFailureAdvisor(walked, append(failureAdvisorOpts(projectRoot, cliRouter, console), core.WithFailureAdvisorSignals(signals))...)),
		core.WithThroughputRecorder(triagecap.Recorder(projectRoot)),
		core.WithCatalogPublisher(catalogPublisher(br, cliRouter)),
		core.WithCLIRouter(cliRouter),
		core.WithRegistrar(registrarMinter{r: phaseregistrar.Registrar{
			Bridge:       walked,
			Prompts:      prm,
			ProfilesDir:  filepath.Join(evolveDir, "profiles"),
			PhasesDir:    filepath.Join(evolveDir, "phases"),
			RegistryPath: mintregistry.Path(projectRoot),
		}}),
	}
	opts = append(opts, phaseObserverOptions(pol.ObserverConfig(), cfg.PhaseRecovery.String(), observerDeps{signals: signals, usage: explainUsage})...)
	var reviewers []core.DeliverableReviewer
	if floor := composedBuildHandoffFloor(pol.WorkflowConfig(), cfg); len(floor) > 0 {
		reviewers = append(reviewers, floor)
	}
	if cfg.EvalGate != config.StageOff {
		reviewers = append(reviewers, evalGateReviewer(cfg.EvalGate, hostFollowupFiler(ld, signals, forbidden, pol.InboxPriorityConfig().ClassOrder)))
	}
	if cfg.ContractGate != config.StageOff {
		rev := deliverable.NewReviewerWithCatalogStageReportSize(
			cfg.ContractGate, catalog, cfg.PhaseIO,
			parseGateStage(gatesCfg.ReportSizeGate), pol.ReportBudgetConfig().HandoffTokens, deliverable.WithSignals(signals))
		contractVerifier = rev
		reviewers = append(reviewers, rev)
	}
	if cfg.TriageCapGate != config.StageOff {
		reviewers = append(reviewers, triagecap.NewReviewer(cfg.TriageCapGate))
	}
	if cfg.TopNGate != config.StageOff {
		reviewers = append(reviewers, topngate.NewReviewer(cfg.TopNGate))
	}
	reviewers = append(reviewers, core.NewHandoffReviewer(pol.HandoffConfig(), handoffReviewCheck))
	if len(reviewers) > 0 {
		opts = append(opts, core.WithReviewer(core.ChainReviewers(reviewers...)))
	}
	if cfg.ContractGate != config.StageOff {
		opts = append(opts, core.WithContractVerifier(deliverable.NewVerifierWithCatalogStage(catalog, cfg.PhaseIO)))
	}
	opts = append(opts, core.WithCatalogRefresher(makeCatalogRefresher(projectRoot, evolveDir, pol.CatalogConfig().RefreshStage)))
	opts = append(opts, core.WithCatalogRefreshStage(catalogRefreshStage(pol)))

	opts = append(opts, core.WithModelCatalogLookup(resolveModelTier))

	opts = append(opts, core.WithDirectivesProvider(makeDirectivesProvider(projectRoot)))

	opts = append(opts, core.WithKB(research.NewFileKBWithRecall(kbRootsAbs(projectRoot), kbRecallK(projectRoot))))

	opts = append(opts, core.WithShipFloor(policyShipFloor))
	opts = append(opts, core.WithRetryConfig(pol.RetryConfig()))
	opts = append(opts, core.WithFailureCountReader(failureCountReader(projectRoot)))
	opts = append(opts, core.WithFailureClassReader(failureClassReader(projectRoot)))
	opts = append(opts, core.WithScopePathResolver(scopePathResolver))
	opts = append(opts, core.WithContinuationResolver(continuationForScope))
	opts = append(opts, core.WithWorkflowConfig(wfCfg))
	opts = append(opts, core.WithChronicleConfig(pol.ChronicleConfig()))
	if fp, err := pol.FailurePolicyConfig(); err == nil {
		opts = append(opts, core.WithFailurePolicy(fp))
	} else {
		fmt.Fprintf(os.Stderr, "[cycle] WARN: malformed failure_policy in policy.json (%v) — using compiled defaults (floor preserved)\n", err)
	}
	opts = append(opts, core.WithWorktreeBase(pol.WorktreeBase()))

	opts = append(opts, compositionOptions()...)
	hostEffects = deliverable.NewHostEffects(catalog, hostInboxClaimer(ld, signals, forbidden))
	opts = append(opts, core.WithHostEffects(hostEffects))
	opts = append(opts, core.WithLaneMenu(laneMenuOf(forbidden)))
	opts = append(opts, core.WithProtectedSurface(protectedMembership))
	opts = append(opts, core.WithSignalCenter(signals))

	return orchDeps{
		Storage:      st,
		Ledger:       ld,
		Orchestrator: core.NewOrchestrator(st, ld, runners, opts...),
		Signals:      signals,
		CloseSignals: closeSignals,
		Bridge:       br,
		Runners:      runners,
		Router:       cliRouter,
	}
}

func stopReviewRecorder(ld core.Ledger) func(cycle int, phase, action, reason string) {
	return func(cycle int, phase, action, reason string) {
		_ = ld.Append(context.Background(), core.LedgerEntry{
			TS:      time.Now().UTC().Format(time.RFC3339),
			Cycle:   cycle,
			Role:    phase,
			Kind:    "stop_review",
			Action:  action,
			Message: reason,
		})
	}
}

func cycleDiagf(format string, args ...any) { fmt.Fprintf(os.Stderr, format, args...) }

func cycleBenchOnEscalation(e bridgechain.Escalation) {
	bridgechain.BenchOnEscalation(e, time.Now, cycleDiagf)
}

func catalogRefreshStage(pol policy.Policy) func() string {
	return func() string { return pol.CatalogConfig().RefreshStage }
}

func failureCountReader(projectRoot string) func(id string) int {
	return func(id string) int {
		n, _ := inboxmover.ReadFailureCount(inboxmover.Options{ProjectRoot: projectRoot}, id)
		return n
	}
}

func failureClassReader(projectRoot string) func(id string) string {
	return func(id string) string {
		return inboxmover.ReadLastFailureClass(inboxmover.Options{ProjectRoot: projectRoot}, id)
	}
}

func continuationForScope(root string, cycle int, scopeIDs []string) *continuation.Continuation {
	return inboxmover.ResolveContinuationForScope(inboxmover.Options{ProjectRoot: root, Stderr: os.Stderr}, cycle, scopeIDs)
}

func hostInboxClaimer(ld inboxmover.LedgerAppender, signals *signalcenter.Center, forbidden func(string) bool) deliverable.Claimer {
	return func(inboxDir string, cycle int, ids []string) error {
		return inboxmover.ClaimPending(inboxmover.Options{InboxDir: inboxDir, Stderr: os.Stderr, Ledger: ld, Signals: signals, IsProtectedPath: forbidden}, cycle, ids)
	}
}

func hostFollowupFiler(ld inboxmover.LedgerAppender, signals *signalcenter.Center, forbidden func(string) bool, classes []string) evalgate.ConsoleFollowupFiler {
	return func(projectRoot string, raw []byte) error {
		_, err := inboxmover.File(inboxmover.Options{ProjectRoot: projectRoot, Stderr: os.Stderr, Ledger: ld, Signals: signals, IsProtectedPath: forbidden, PriorityClasses: classes, Origin: inboxbatch.OriginCycle}, raw)
		return err
	}
}

func kbRecallK(projectRoot string) int {
	return policy.LoadOrDefaults(filepath.Join(projectRoot, ".evolve", "policy.json"), os.Stderr).ResearchConfig().RecallK
}

func kbRootsAbs(projectRoot string) []string {
	raw := research.SearchPathsFromEnv(policy.LoadOrDefaults(filepath.Join(projectRoot, ".evolve", "policy.json"), os.Stderr).PathsConfig())
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		if filepath.IsAbs(p) {
			out = append(out, p)
			continue
		}
		out = append(out, filepath.Join(projectRoot, p))
	}
	return out
}

func cycleContext(goalHash, goalText string) map[string]string {
	ctx := map[string]string{
		"commit_message": fmt.Sprintf("evolve-cycle: goal=%s", goalHash),
	}
	if goalText != "" {
		ctx["goal"] = goalText
	}
	return ctx
}

func registerBuiltinSpecRunners(runners map[core.Phase]core.PhaseRunner, builtinCat phasespec.Catalog, base specrunner.Config, warn io.Writer) {
	for _, s := range builtinCat.All() {
		if s.KindOrDefault() != "llm" || s.RoleOrDefault() == phasespec.RoleControl {
			continue
		}
		if _, exists := runners[core.Phase(s.Name)]; exists {
			continue
		}
		if _, err := base.Prompts.Agent(s.AgentName()); err != nil {
			fmt.Fprintf(warn, "[phases] WARN selectable spec phase %q (kind:llm) has no runner and no persona %s.md — not dispatchable until a persona is added\n", s.Name, s.AgentName())
			continue
		}
		runners[core.Phase(s.Name)] = specrunner.New(s, base)
	}
}

type registrarMinter struct{ r phaseregistrar.Registrar }

func (m registrarMinter) Register(cfg phaseconfig.PhaseConfig) (phasespec.PhaseSpec, core.PhaseRunner, error) {
	res, err := m.r.Register(cfg)
	if err != nil {
		return phasespec.PhaseSpec{}, nil, err
	}
	return res.Spec, res.Runner, nil
}

func evalGateReviewer(stage config.Stage, file evalgate.ConsoleFollowupFiler) core.DeliverableReviewer {
	return evalgate.NewReviewer(stage, evalgate.WithConsoleFollowupFiler(file), evalgate.WithScopePathResolver(scopePathResolver))
}

func scopePathResolver(projectRoot, taskID string) string {
	st := inboxmover.ResolveDispatchState(inboxmover.Options{ProjectRoot: projectRoot}, taskID)
	if st.State != inboxmover.StatePending && st.State != inboxmover.StateProcessing {
		return ""
	}
	return st.Path
}

func failureAdvisorOpts(projectRoot string, r *cliroute.Router, console io.Writer) []core.FailureAdvisorOption {
	cli := failureAdvisorCLI(projectRoot, r, console)
	if cli == "" {
		return nil
	}
	return []core.FailureAdvisorOption{core.WithFailureAdvisorCLI(cli)}
}

func failureAdvisorCLI(projectRoot string, r *cliroute.Router, console io.Writer) string {
	const agent = "failure-advisor"
	res, err := r.ResolveRole(agent, resolvellm.Options{ProjectRoot: projectRoot, GitRoot: projectRoot})
	if err != nil {
		fmt.Fprintf(console, "[cycle] WARN failure advisor has no route (agent %s), so it keeps the compiled default: %v\n", agent, err)
		return ""
	}
	return res.CLI
}

func loadPhaseCatalog(projectRoot, registryPath string, prm *prompts.Loader, emit signalcenter.Listener) (phasespec.Catalog, phasespec.Catalog, []phasespec.PhaseSpec) {
	builtinCat, builtinErr := phasespec.Load(registryPath)
	if builtinErr != nil {
		emit(phasespec.RoutingDefectEvent("loadPhaseCatalog", fmt.Sprintf("builtin registry load failed (%v); builtin spec-runners not registered", builtinErr)))
	}
	userSpecs, discWarns := discoverUserSpecsClamped(projectRoot, prm)
	catalog, mergeWarns := builtinCat.Merge(userSpecs)
	emitRoutingDefects(emit, "loadPhaseCatalog", append(discWarns, mergeWarns...))
	return catalog, builtinCat, userSpecs
}

func emitRoutingDefects(emit signalcenter.Listener, origin string, warnings []string) {
	for _, w := range warnings {
		emit(phasespec.RoutingDefectEvent(origin, w))
	}
}

func documentSpecPtr(cfg config.RoutingConfig) *config.DeliverableKindSpec {
	spec, ok := cfg.DocumentSpec()
	if !ok {
		return nil
	}
	return &spec
}

func laneMenuOf(forbidden func(string) bool) core.LaneMenuFn {
	return func(inboxRoot string, items []inboxbatch.Item) []inboxbatch.Item {
		return inboxmover.PartitionLaneMenu(inboxmover.Options{InboxDir: inboxRoot, Stderr: io.Discard}, items, forbidden).Ready
	}
}
