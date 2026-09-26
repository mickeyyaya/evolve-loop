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
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/observer"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
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
	"github.com/mickeyyaya/evolve-loop/go/internal/research"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/topngate"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

// runCycle implements `evolve cycle <subcommand>`: run | reset | timing | outputs.
func runCycle(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve cycle: missing subcommand (try: run | reset | timing)")
		return 10
	}
	switch args[0] {
	case "run":
		return runCycleRun(args[1:], stdout, stderr)
	case "reset":
		return runCycleReset(args[1:], stdout, stderr)
	case "timing":
		return runCycleTiming(args[1:], stdout, stderr)
	case "outputs":
		return runCycleOutputs(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve cycle: unknown subcommand %q\n", args[0])
		return 10
	}
}

// runCycleReset seals an unfinished cycle through core.SealCycle; the
// complement of `evolve loop --resume`.
func runCycleReset(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve cycle reset", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		projectRoot string
		evolveDir   string
		reason      string
		dryRun      bool
		force       bool
	)
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to the project root (default cwd)")
	fs.StringVar(&evolveDir, "evolve-dir", "", "path to .evolve/ state directory (default <project-root>/.evolve)")
	fs.StringVar(&reason, "reason", "operator-requested reset", "reason recorded in the seal manifest + ledger")
	fs.BoolVar(&dryRun, "dry-run", false, "print the seal plan without mutating any state")
	fs.BoolVar(&force, "force", false, "seal even when the cycle's run lease is still fresh (override a live, heartbeating owner — last resort)")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	// SealCycle's "workspace inside projectRoot" check needs both sides absolute.
	projectRoot = paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve cycle reset: WARN: %s\n", m)
	})
	if evolveDir == "" {
		evolveDir = filepath.Join(projectRoot, ".evolve")
	}

	// The liveness fence is SealCycle's lease heartbeat. The dispatcher's
	// .evolve/.lock is per cycle and released between cycles, so it proves nothing.
	res, err := core.SealCycle(context.Background(), ledger.New(evolveDir), core.SealOptions{
		EvolveDir:   evolveDir,
		ProjectRoot: projectRoot,
		Reason:      reason,
		DryRun:      dryRun,
		Force:       force,
		// A dead owner pid seals without --force even before its heartbeat ages out.
		PidAlive: pidAlive,
	})
	if err != nil {
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
			return 1
		default:
			fmt.Fprintf(stderr, "evolve cycle reset: %v\n", err)
			return 1
		}
	}
	if res.ForcedOverLiveOwner {
		fmt.Fprintf(stderr, "evolve cycle reset: WARN --force sealed cycle %d while its lease was still FRESH (pid %d, heartbeat %s ago) — a live owner was overridden.\n",
			res.SealedCycleID, res.LeaseOwnerPID, res.LeaseHeartbeatAge.Round(time.Second))
	}

	verb := "sealed"
	if res.DryRun {
		verb = "would seal"
	}
	fmt.Fprintf(stdout, "%s cycle %d (phase=%s) → %s; next cycle %d\n",
		verb, res.SealedCycleID, res.SealedPhase, res.ArchiveDir, res.NextCycle)
	return 0
}

func runCycleRun(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve cycle run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		projectRoot  string
		goalHash     string
		goalText     string
		evolveDir    string
		simulate     bool
		bypassPolicy bool
	)
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to the project root (default cwd)")
	fs.StringVar(&goalHash, "goal-hash", "", "8-char SHA256 of the goal (required)")
	fs.StringVar(&goalText, "goal", "", "human-readable goal text (threaded to Scout + the routing advisor as Context[\"goal\"]); optional")
	fs.StringVar(&evolveDir, "evolve-dir", "", "path to .evolve/ state directory (default <project-root>/.evolve)")
	fs.BoolVar(&simulate, "simulate", false, "no-LLM walk: every phase returns PASS without calling out (for parity-audit harness)")
	fs.BoolVar(&bypassPolicy, "bypass-policy", false, "use --bypass-policy to bypass policy.json pin enforcement for every phase this run (operator escape hatch)")
	args = stripRemovedBudgetFlags(args, func(m string) {
		fmt.Fprintf(stderr, "evolve cycle run: WARN: %s\n", m)
	})
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if goalHash == "" {
		fmt.Fprintln(stderr, "evolve cycle run: --goal-hash is required")
		return 10
	}
	// Absolutize before deriving any path: a relative root makes worktree-phase
	// artifact paths diverge between the agent's cwd and the bridge's.
	projectRoot = paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve cycle run: WARN: %s\n", m)
	})
	if evolveDir == "" {
		evolveDir = filepath.Join(projectRoot, ".evolve")
	}

	if !simulate {
		gcOrphanSessions("cycle-start", stderr)
	}

	var d orchDeps
	if simulate {
		d = wireSimulateOrchestrator(projectRoot, evolveDir, stderr)
	} else {
		d = wireOrchestratorDeps(projectRoot, evolveDir, stderr)
	}
	var lifecycleLedger inboxmover.LedgerAppender = d.Ledger
	orch, signals := d.Orchestrator, d.Signals
	defer d.Signals.Flush()
	cycleEnv := filterEvolveEnv(os.Environ())
	result, err := orch.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot:           projectRoot,
		GoalHash:              goalHash,
		Env:                   cycleEnv,
		Context:               cycleContext(goalHash, goalText),
		DisableWorkspaceGuard: disableWorkspaceGuardForTest,
		BypassPolicy:          bypassPolicy,
	})
	if err != nil {
		// Fleet lanes run this entrypoint as a subprocess and fleet.Result carries no
		// cycle or workspace, so a lane's FAIL reaches the inbox lifecycle only here.
		var clf *core.ErrCycleLevelFailure
		if errors.As(err, &clf) {
			warnCycleFailureOutcome(stderr, result.Cycle, applyCycleFailureOutcome(projectRoot, evolveDir, result.Cycle, stderr, lifecycleLedger, signals))
		}
		fmt.Fprintf(stderr, "evolve cycle run: %v\n", err)
		return 1
	}
	buf, _ := json.MarshalIndent(result, "", "  ")
	fmt.Fprintln(stdout, string(buf))
	// The exit code is a lane's only channel to its parent wave, so a halting
	// SystemFailure gets its own code and writes the dossier and P0 item here.
	if sf := result.SystemFailure; sf != nil && sf.Halt {
		return haltOnSystemFailure(evolveDir, projectRoot, result.Cycle, cycleWorkspace(projectRoot, result.Cycle), sf, stderr, signals, systemFailureRule)
	}
	// A FAIL verdict without an error, or a no-work hand-off; the error branch
	// above already returned, so each applies once.
	if applied, err := closeoutCycleOutcome(result, projectRoot, evolveDir, stderr, lifecycleLedger, signals); err != nil {
		fmt.Fprintf(stderr, "evolve cycle run: WARN: could not apply cycle %d %s to the inbox: %v\n", result.Cycle, applied, err)
	}
	return cycleRunExitCode(result)
}

// closeoutCycleOutcome is the one post-result closeout every root makes: the
// failure walk on FAIL, the no-work hand-off for a lane that answered its
// scope, else nothing. It returns what it applied and the walk's error for the
// caller to WARN; a lifecycle hiccup never changes an exit code or a batch.
func closeoutCycleOutcome(result core.CycleResult, projectRoot, evolveDir string, stderr io.Writer, lifecycle inboxmover.LedgerAppender, signals *signalcenter.Center) (applied string, err error) {
	switch {
	case result.FinalVerdict == cyclestate.VerdictFAIL:
		return "failure outcome", applyCycleFailureOutcome(projectRoot, evolveDir, result.Cycle, stderr, lifecycle, signals)
	case core.IsTriageNoWorkResult(result):
		return "no-work hand-off", applyCycleNoWorkOutcome(projectRoot, result.Cycle, stderr, lifecycle, signals)
	}
	return "", nil
}

// applyCycleNoWorkOutcome hands a no-work lane's answered scoped items to the
// console and releases its claims; a cycle without a lane pin has none.
func applyCycleNoWorkOutcome(projectRoot string, cycle int, stderr io.Writer, lifecycle inboxmover.LedgerAppender, signals *signalcenter.Center) error {
	_, err := cycleoutcome.ApplyNoWork(cycleoutcome.NoWorkInputs{
		ProjectRoot: projectRoot,
		Workspace:   cycleWorkspace(projectRoot, cycle),
		Cycle:       cycle,
		Stderr:      stderr,
	}.WithLedger(lifecycle).WithSignals(signals))
	return err
}

// applyCycleFailureOutcome walks the failed cycle's committed ids through the
// inbox failure lifecycle on the root's ledger and Signal Center. A nil ledger
// (the --simulate root) falls back to the mover's own unobserved file ledger.
func applyCycleFailureOutcome(projectRoot, evolveDir string, cycle int, stderr io.Writer, lifecycle inboxmover.LedgerAppender, signals *signalcenter.Center) error {
	_, err := cycleoutcome.ApplyFailure(cycleoutcome.FailureInputsFor(
		projectRoot, evolveDir, cycleWorkspace(projectRoot, cycle), cycle, stderr,
	).WithLedger(lifecycle).WithSignals(signals))
	return err
}

// warnCycleFailureOutcome is the cycle-run root's voice for a failed walk.
func warnCycleFailureOutcome(stderr io.Writer, cycle int, err error) {
	if err != nil {
		fmt.Fprintf(stderr, "evolve cycle run: WARN: could not apply cycle %d failure outcome to the inbox: %v\n", cycle, err)
	}
}

// filterEvolveEnv forwards to cmdutil.FilterEvolveEnv, the one definition the
// internal/cli groups share.
func filterEvolveEnv(environ []string) map[string]string {
	return cmdutil.FilterEvolveEnv(environ)
}

// orchDeps is the wired orchestrator plus the storage and ledger handles its
// callers query, so none re-resolves evolveDir.
type orchDeps struct {
	Storage      core.Storage
	Ledger       rootLedger
	Orchestrator *core.Orchestrator
	// Signals is always built; a nil Center is a test affordance only.
	Signals *signalcenter.Center
	// Bridge carries Signals into every engine it builds.
	Bridge *bridge.Adapter
	// Runners backs the per-runner Signals wiring proof; no production reader.
	Runners map[core.Phase]core.PhaseRunner
}

// rootLedger is one object serving the core port and the inbox mover's
// lifecycle seam, so the failed-cycle walk appends through the observed ledger.
type rootLedger interface {
	core.Ledger
	inboxmover.LedgerAppender
}

// newRootSignalCenter is the one sink topology every root builds, the loop
// tests' stub root included: signals.ndjson (the cycle workspace's, or
// <evolveDir>'s for cycle-less signals) plus the console at WARN and above.
func newRootSignalCenter(projectRoot, evolveDir string, console io.Writer) *signalcenter.Center {
	signals := signalcenter.New(signalcenter.WithPID(os.Getpid()))
	signals.Subscribe(signals.NDJSONSink(func(cycle int) string {
		if cycle == 0 {
			return filepath.Join(evolveDir, signalcenter.StreamFileName)
		}
		return filepath.Join(core.RunWorkspacePath(projectRoot, cycle), signalcenter.StreamFileName)
	}))
	signals.Subscribe(signalcenter.ConsoleSink(console))
	return signals
}

// wireOrchestratorDeps builds the production orchestrator and returns it with
// its storage, ledger, Signal Center, bridge and runners.
func wireOrchestratorDeps(projectRoot, evolveDir string, console io.Writer) orchDeps {
	// Pin the model-catalog dir to this .evolve, which the cycle-start refresher
	// writes; EVOLVE_PROJECT_ROOT may name another tree.
	if evolveDir != "" {
		d := evolveDir
		bridge.SetModelCatalogDirFn(func() string { return d })
	}
	// Built first and unconditionally: the bridge takes the Center at construction.
	// TestNilSignalCenterRootsArePinned lists the roots that may skip it.
	signals := newRootSignalCenter(projectRoot, evolveDir, console)
	// The ledger precedes the bridge so the stop-review callback can append to it.
	st := storage.New(evolveDir)
	ld := ledger.New(evolveDir, ledger.WithSignals(signals))

	br := bridge.NewDefault(projectRoot, signals)
	br.SetOnStopReview(func(cycle int, phase, action, reason string) {
		_ = ld.Append(context.Background(), core.LedgerEntry{
			TS:      time.Now().UTC().Format(time.RFC3339),
			Cycle:   cycle,
			Role:    phase,
			Kind:    "stop_review",
			Action:  action,
			Message: reason,
		})
	})
	prm := cmdutil.NewPromptsLoader(projectRoot)

	// The root is the sole reader of routing env and config. Loaded before the
	// runners so cfg.PhaseIO reaches the reconcile rung; warnings ride the Center.
	registryPath := config.RegistryPath(projectRoot)
	loader := wiredRoutingConfigLoader(signals)
	cfg, _ := loader.Load(registryPath, filterEvolveEnv(os.Environ()))

	// policy.json can only add mandatory phases. A malformed policy WARNs here and
	// fails loudly at the first dispatch, where the runner reloads it for pins.
	var shipFloor []string // nil ⇒ router.DefaultShipFloor
	pol, policyErr := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if policyErr != nil {
		fmt.Fprintf(os.Stderr, "[policy] WARN %v (mandatory merge skipped; fails loudly at dispatch)\n", policyErr)
		pol = policy.Policy{}
	} else {
		cfg.Mandatory = pol.MergeMandatory(cfg.Mandatory)
		if floor, overridden := pol.FloorPhases(); overridden {
			shipFloor = floor
		}
		// failure_floor is the one surface for the audit-FAIL route, folded the same
		// way as router.PolicyForProject.
		cfg.AuditFailRoutesTo = router.FailureRouteFromPolicy(pol)
	}
	swCfg := swarmrunner.Config{Stage: pol.SwarmConfig().Stage, PortBase: pol.SwarmConfig().PortBase, WorktreeBase: pol.WorktreeBase()}
	gatesCfg := pol.GatesConfig()
	routerCfg := pol.RouterConfig()
	recoveryCfg := pol.RecoveryConfig()
	// The policy dials resolve through the Loader's ladders, so a typo'd word warns
	// CONFIG_UNKNOWN_VALUE; recoveryCfg survives only for this projection.
	cfg, _ = loader.ApplyPolicyStages(cfg, policyStagesOf(gatesCfg, recoveryCfg, routerCfg, pol.ParallelEvaluateConfig()))
	wfCfg := pol.WorkflowConfig()

	// Set the universal-fallback seams once, before the constructors, so a phase
	// whose CLI chain is absent on this host routes to a present LLM. Discovery is
	// a memoized bridge.Doctor probe run lazily by the first phase that needs it.
	runner.DefaultUniversalFallback = wfCfg.UniversalFallback
	if wfCfg.UniversalFallback {
		var discOnce sync.Once
		var discovered []string
		runner.DefaultDiscoverCLIsFn = func() []string {
			discOnce.Do(func() {
				// Bounded: a hung `<cli> --version` would wedge this sync.Once and stall the
				// loop. A timeout degrades to no discovery, which fails loud.
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				rep, _ := gobridge.NewEngine(gobridge.Deps{}).Doctor(ctx, "", false)
				seenFam := map[string]bool{}
				for _, r := range rep.Results {
					fam := llmroute.Family(r.CLI)
					if seenFam[fam] || !r.Binary.Present || r.Verdict == "blocked" {
						continue
					}
					seenFam[fam] = true
					discovered = append(discovered, fam+"-tmux")
				}
				// The operator's family ban applies to this last-resort tail only.
				discovered = llmroute.ExcludeFamilies(discovered, wfCfg.UniversalFallbackExclude)
			})
			return discovered
		}
	}

	// The CLI/tier fallback chain wraps the bridge handle once, and every consumer
	// below launches through walked. The runner and the advisor walk their own
	// chains, so they pass straight through.
	diagf := func(format string, args ...any) { fmt.Fprintf(os.Stderr, format, args...) }
	discover := func() []string {
		if runner.DefaultDiscoverCLIsFn == nil {
			return nil
		}
		return runner.DefaultDiscoverCLIsFn()
	}
	walked := bridgechain.New(br,
		bridgechain.DefaultPlanResolver(filepath.Join(evolveDir, "profiles"), discover, nil, time.Now, diagf),
		bridgechain.WithBench(func(root, ws, cli string, start time.Time, env map[string]string) {
			bridgechain.BenchOnEscalation(root, ws, cli, start, env, time.Now, diagf)
		}),
		bridgechain.WithLog(diagf))

	// Every BaseRunner's verdict engine reads the contract verifier through this
	// accessor, so the engine classifies the bytes the gate approves. Gate off
	// keeps the Null-Object PlainVerifier; the gate's Reviewer replaces it below.
	var contractVerifier runner.ContractVerifier = deliverable.PlainVerifier{PhaseIO: cfg.PhaseIO}
	verifierOf := func() runner.ContractVerifier { return contractVerifier }
	var hostEffects core.HostEffects
	hostEffectsOf := func() core.HostEffects { return hostEffects }

	runners := map[core.Phase]core.PhaseRunner{
		core.PhaseIntent: intent.New(intent.Config{Bridge: walked, Prompts: prm, ContractVerifier: verifierOf, HostEffects: hostEffectsOf, CompactPrompts: cfg.CompactPrompts}),
		// Scout and Build are swarm-eligible; below advisory the Decorator delegates.
		// See ADR-0032.
		core.PhaseScout:        swarmrunner.New(scout.New(scout.Config{Bridge: walked, Prompts: prm, ContractVerifier: verifierOf, HostEffects: hostEffectsOf, PhaseIO: cfg.PhaseIO, CompactPrompts: cfg.CompactPrompts}), walked, swarm.ModeReader, swCfg),
		core.PhaseTriage:       triage.New(triage.Config{Bridge: walked, Prompts: prm, ContractVerifier: verifierOf, HostEffects: hostEffectsOf, PhaseIO: cfg.PhaseIO, CompactPrompts: cfg.CompactPrompts}),
		core.PhaseTDD:          tdd.New(tdd.Config{Bridge: walked, Prompts: prm, ContractVerifier: verifierOf, HostEffects: hostEffectsOf, CompactPrompts: cfg.CompactPrompts}),
		core.PhaseBuildPlanner: buildplanner.New(buildplanner.Config{Bridge: walked, Prompts: prm, ContractVerifier: verifierOf, HostEffects: hostEffectsOf}).BaseRunner(),
		core.PhaseBuild:        swarmrunner.New(build.New(build.Config{Bridge: walked, Prompts: prm, ContractVerifier: verifierOf, HostEffects: hostEffectsOf, PhaseIO: cfg.PhaseIO, CompactPrompts: cfg.CompactPrompts}), walked, swarm.ModeWriter, swCfg),
		core.PhaseAudit:        audit.NewDefaultWithStageCompactSpec(walked, prm, cfg.PhaseIO, cfg.CompactPrompts, documentSpecPtr(cfg), audit.WithContractVerifier(verifierOf), audit.WithHostEffects(hostEffectsOf), audit.WithSignals(func() *signalcenter.Center { return signals })),
		// The ship-bind manifest gate is operator-activatable via gates.manifest_gate.
		core.PhaseShip:  ship.New(ship.Config{Runner: sysexec.DefaultRunner, PhaseIO: cfg.PhaseIO, ManifestGate: gatesCfg.ManifestGate, RepoContractGate: gatesCfg.RepoContractGate, Signals: signals}),
		core.PhaseRetro: retro.New(retro.Config{Bridge: walked, Prompts: prm, Model: "auto", CompactPrompts: cfg.CompactPrompts}),
		// The debugger diagnoses a novel ShipError; optional, never on the spine.
		core.PhaseDebugger: debugger.New(debugger.Config{Bridge: walked, Prompts: prm, ContractVerifier: verifierOf, HostEffects: hostEffectsOf, CompactPrompts: cfg.CompactPrompts}),
	}

	// User phases merge over the built-in catalog; only valid specs are routed and
	// get a spec runner.
	builtinCat, builtinErr := phasespec.Load(registryPath)
	if builtinErr != nil {
		// Non-fatal but loud: with no registry no builtin spec runner is wired, so a
		// selectable phase would abort at dispatch.
		fmt.Fprintf(os.Stderr, "[phases] WARN builtin registry load failed (%v); builtin spec-runners not registered\n", builtinErr)
	}
	userSpecs, discWarns := discoverUserSpecsClamped(projectRoot, prm)
	catalog, mergeWarns := builtinCat.Merge(userSpecs)
	for _, w := range append(discWarns, mergeWarns...) {
		fmt.Fprintf(os.Stderr, "[phases] WARN %s\n", w)
	}
	// Catalog-aware so user and minted phases get their spec-derived contract.
	br.SetContractResolver(phasecontract.NewCatalogResolver(catalog.Get))
	// catalog.Get is bound to this pre-mint catalog value; the catalog publisher
	// below re-binds the resolver after each mid-cycle mint.
	wireBridgeStages(br, cfg)
	for _, w := range phasespec.ApplyUserRouting(&cfg, userSpecs, builtinCat) {
		fmt.Fprintf(os.Stderr, "[phases] WARN %s\n", w)
	}
	// Must use ApplyUserRouting's catalog-aware validator: the bare
	// ValidateUserSpec rejects optional built-ins such as memo, so routing would
	// plan a phase that never dispatches.
	for _, s := range catalog.UserPhases() {
		if len(phasespec.ValidateUserSpecWithCatalog(s, builtinCat)) > 0 {
			continue // ApplyUserRouting already warned + skipped it; no dead runner
		}
		if _, exists := runners[core.Phase(s.Name)]; !exists {
			runners[core.Phase(s.Name)] = specrunner.New(s, specrunner.Config{Bridge: walked, Prompts: prm, ContractVerifier: verifierOf, HostEffects: hostEffectsOf})
		}
	}
	// Spec-runner fallback so every advisor-selectable builtin phase dispatches.
	registerBuiltinSpecRunners(runners, builtinCat, specrunner.Config{Bridge: walked, Prompts: prm, ContractVerifier: verifierOf, HostEffects: hostEffectsOf}, os.Stderr)
	// The routing advisor resolves {cli, model} like a phase: router profile, then
	// policy. Plan decisions use the deep tier. A benched family falls back to
	// claude; with claude benched too the advisor degrades to the static spine.
	advCLI, advModel, advHealthy := resolveRouterDispatchHealthy(evolveDir, decisionPlan, benchedFamilies(projectRoot), routerCfg)
	if !advHealthy {
		fmt.Fprintf(os.Stderr, "[router] WARN router family and the claude fallback are both benched — advisor will degrade to the static spine\n")
	}
	var advPersona string
	if rp, perr := prm.Agent("evolve-router"); perr == nil {
		advPersona = rp.Body
	} else {
		fmt.Fprintf(os.Stderr, "[router] WARN persona evolve-router.md not loaded (%v); advisor uses legacy inline framing\n", perr)
	}
	advisor := core.NewPhaseAdvisor(walked,
		core.WithProposerCLI(advCLI),
		core.WithProposerModel(advModel),
		core.WithPersona(advPersona),
		core.WithDepthCheck(core.AdvisorDepthExceeded),
		core.WithAdvisorSignals(signals),
	)
	strategy := router.Select(cfg, advisor)
	// The advisor also plans the whole cycle; the orchestrator consults it only at
	// Stage>=Advisory with DynamicLLM, so wiring it here costs nothing otherwise.
	opts := []core.Option{
		core.WithRouting(cfg, strategy),
		core.WithCatalog(catalog),
		core.WithPlanner(advisor),
		// Best-effort and gated on cfg.PhaseRecovery=enforce; below that it never
		// dispatches.
		core.WithFailureAdviser(core.NewFailureAdvisor(walked, failureAdvisorOpts(projectRoot)...)),
		// Feeds state.json:triageThroughput, the window the triage clamp bounds with.
		core.WithThroughputRecorder(triagecap.Recorder(projectRoot)),
		// Re-bind the bridge's contract resolver on each mid-cycle mint, so a minted
		// phase resolves its contract in the same cycle.
		core.WithCatalogPublisher(catalogPublisher(br)),
		core.WithRegistrar(registrarMinter{r: phaseregistrar.Registrar{
			Bridge:       walked,
			Prompts:      prm,
			ProfilesDir:  filepath.Join(evolveDir, "profiles"),
			PhasesDir:    filepath.Join(evolveDir, "phases"),
			RegistryPath: mintregistry.Path(projectRoot), // the tree-diff guard reads it under projectRoot
		}}),
	}
	// Auto-spawn the per-phase observer unless policy.json disables it.
	observerCfg := pol.ObserverConfig()
	if *observerCfg.Autospawn {
		ca := observer.NewCoreAdapter(observerCfg)
		ca.RecoveryStage = cfg.PhaseRecovery.String()
		ca.Signals = func() *signalcenter.Center { return signals } // the adapter's own faults are observer.warning signals
		opts = append(opts, core.WithObserver(ca))
	}
	// Every deliverable gate chains behind one reviewer, since WithReviewer takes
	// one. Each is gated on its own and fails open on ambiguity.
	var reviewers []core.DeliverableReviewer
	// The build floor runs first: its rejection carries the defect list the
	// correction ladder needs. It is the one real go-test run per changed package;
	// the advisory post-build selfcheck skips its duplicate when this is enforced.
	if pol.WorkflowConfig().BuildFloorEnforced {
		checks := productionBuildFloorChecks
		// A document cycle's solutions/<slug>/ is judged by the same floor seam.
		if spec, ok := cfg.DocumentSpec(); ok {
			checks = core.ChainBuildFloorChecks(productionBuildFloorChecks, core.SolutionFloorChecks(spec))
		}
		reviewers = append(reviewers, core.NewBuildFloorReviewer(checks))
	}
	if cfg.EvalGate != config.StageOff {
		reviewers = append(reviewers, evalgate.NewReviewer(cfg.EvalGate))
	}
	if cfg.ContractGate != config.StageOff {
		// Catalog-aware, so the gate enforces what `evolve phase verify` derives from
		// phase.json. The report-size gate rides it as its own dial.
		rev := deliverable.NewReviewerWithCatalogStageReportSize(
			cfg.ContractGate, catalog, cfg.PhaseIO,
			parseGateStage(gatesCfg.ReportSizeGate), pol.ReportBudgetConfig().HandoffTokens, deliverable.WithSignals(signals))
		contractVerifier = rev
		reviewers = append(reviewers, rev)
	}
	if cfg.TriageCapGate != config.StageOff {
		// Capacity clamp, after the contract gate: well-formedness first.
		reviewers = append(reviewers, triagecap.NewReviewer(cfg.TriageCapGate))
	}
	if cfg.TopNGate != config.StageOff {
		// Task-binding clamp, after the contract gate: a build whose task is outside
		// triage top_n, or a TDD that authors tests under an empty or disjoint top_n,
		// aborts before later phases spend on it.
		reviewers = append(reviewers, topngate.NewReviewer(cfg.TopNGate))
	}
	if len(reviewers) > 0 {
		opts = append(opts, core.WithReviewer(core.ChainReviewers(reviewers...)))
	}
	if cfg.ContractGate != config.StageOff {
		// The salvage rung's re-check shares the gate's resolution, so they agree on
		// well-formed. Wired at every stage for shadow telemetry; execution stays
		// gated on EVOLVE_PHASE_RECOVERY=enforce.
		opts = append(opts, core.WithContractVerifier(deliverable.NewVerifierWithCatalogStage(catalog, cfg.PhaseIO)))
	}
	// Cycle-start live model-catalog refresh, best-effort under its stage.
	opts = append(opts, core.WithCatalogRefresher(makeCatalogRefresher(projectRoot, evolveDir, pol.CatalogConfig().RefreshStage)))
	// The refresher's stage, stamped into each catalog_refresh ledger entry.
	opts = append(opts, core.WithCatalogRefreshStage(func() string { return pol.CatalogConfig().RefreshStage }))

	// Without this lookup router.ClampPlanModelRouting cannot clear an
	// unresolvable {cli, tier} and silently does nothing.
	opts = append(opts, core.WithModelCatalogLookup(resolveModelTier))

	// The only resolver of operator-directives config; the orchestrator consumes
	// a snapshot each cycle.
	opts = append(opts, core.WithDirectivesProvider(makeDirectivesProvider(projectRoot)))

	// The recall bound resolves from policy.json research.recall_k; a knob that
	// never reaches this construction is dead config.
	opts = append(opts, core.WithKB(research.NewFileKBWithRecall(kbRootsAbs(projectRoot), kbRecallK(projectRoot))))

	opts = append(opts, core.WithShipFloor(shipFloor))
	opts = append(opts, core.WithRetryConfig(pol.RetryConfig()))
	// Retry-tier escalation reads the durable per-item failure counter.
	opts = append(opts, core.WithFailureCountReader(func(id string) int {
		n, _ := inboxmover.ReadFailureCount(inboxmover.Options{ProjectRoot: projectRoot}, id)
		return n
	}))
	// Continuation-on-fail adopts a prior FAILed attempt's salvage for this scope;
	// a planner-pinned lane resolves from its lane-scope todo ids.
	opts = append(opts, core.WithScopePathResolver(scopePathResolver))
	opts = append(opts, core.WithContinuationResolver(func(root string, cycle int, scopeIDs []string) *continuation.Continuation {
		// The live-scope guard's refusal is the operator's only sign that a ghost
		// binding was caught, so it must reach stderr.
		return inboxmover.ResolveContinuationForScope(inboxmover.Options{ProjectRoot: root, Stderr: os.Stderr}, cycle, scopeIDs)
	}))
	opts = append(opts, core.WithWorkflowConfig(wfCfg))
	opts = append(opts, core.WithChronicleConfig(pol.ChronicleConfig()))
	// A malformed failure_policy keeps the compiled defaults; the floor holds.
	if fp, err := pol.FailurePolicyConfig(); err == nil {
		opts = append(opts, core.WithFailurePolicy(fp))
	} else {
		fmt.Fprintf(os.Stderr, "[cycle] WARN: malformed failure_policy in policy.json (%v) — using compiled defaults (floor preserved)\n", err)
	}
	opts = append(opts, core.WithWorktreeBase(pol.WorktreeBase()))

	// A clean fleet rebase carries the audit verdict forward instead of
	// re-auditing; every closure fails closed.
	opts = append(opts, compositionOptions()...)
	hostEffects = deliverable.NewHostEffects(catalog, hostInboxClaimer(ld, signals))
	opts = append(opts, core.WithHostEffects(hostEffects))
	opts = append(opts, core.WithSignalCenter(signals))

	return orchDeps{
		Storage:      st,
		Ledger:       ld,
		Orchestrator: core.NewOrchestrator(st, ld, runners, opts...),
		Signals:      signals,
		Bridge:       br,
		Runners:      runners,
	}
}

// hostInboxClaimer claims through the same floor as `evolve inbox-mover claim`.
func hostInboxClaimer(ld inboxmover.LedgerAppender, signals *signalcenter.Center) deliverable.Claimer {
	return func(inboxDir string, cycle int, ids []string) error {
		return inboxmover.ClaimPending(inboxmover.Options{InboxDir: inboxDir, Stderr: os.Stderr, Ledger: ld, Signals: signals, IsProtectedPath: guards.IsProtectedScope}, cycle, ids)
	}
}

// kbRecallK resolves the KB recall bound from policy.json. An unreadable
// policy keeps the default rather than disabling recall.
func kbRecallK(projectRoot string) int {
	pol, _ := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	return pol.ResearchConfig().RecallK
}

// kbRootsAbs resolves the KB search roots against the project root, so the
// corpus read does not depend on the process cwd.
func kbRootsAbs(projectRoot string) []string {
	pol, _ := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	raw := research.SearchPathsFromEnv(pol.PathsConfig())
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

// cycleContext seeds CycleRequest.Context with the commit message and, when
// given, Context["goal"], the key Scout and the routing advisor read.
func cycleContext(goalHash, goalText string) map[string]string {
	ctx := map[string]string{
		"commit_message": fmt.Sprintf("evolve-cycle: goal=%s", goalHash),
	}
	if goalText != "" {
		ctx["goal"] = goalText
	}
	return ctx
}

// routerDecisionType selects the advisor decision a dispatch resolves for:
// plan and re-plan want the deep tier, propose and judge the fast one.
type routerDecisionType int

const (
	decisionPlan    routerDecisionType = iota // initial whole-cycle plan (deep)
	decisionRePlan                            // post-scout re-plan (deep)
	decisionPropose                           // reactive per-transition tweak (fast)
	decisionJudge                             // route-quality judge (fast)
)

// resolveRouterDispatchFor applies RouterPolicy's per-decision model override
// to the base dispatch; the CLI is the same for every type.
func resolveRouterDispatchFor(evolveDir string, dt routerDecisionType, rc policy.RouterPolicy) (cli, model string) {
	cli, model = resolveRouterDispatch(evolveDir, rc)
	switch dt {
	case decisionPlan, decisionRePlan:
		if rc.PlanModel != "" {
			model = rc.PlanModel
		}
	case decisionPropose, decisionJudge:
		if rc.ProposeModel != "" {
			model = rc.ProposeModel
		}
	}
	return cli, model
}

// resolveRouterDispatchHealthy falls back to claude-tmux when the chosen family
// is benched. With claude benched too it returns the base dispatch and
// ok=false, and the advisor degrades to the static spine.
func resolveRouterDispatchHealthy(evolveDir string, dt routerDecisionType, benched map[string]bool, rc policy.RouterPolicy) (cli, model string, ok bool) {
	cli, model = resolveRouterDispatchFor(evolveDir, dt, rc)
	if !benched[llmroute.Family(cli)] {
		return cli, model, true
	}
	if benched["claude"] {
		// A usable dispatch even with ok=false, so a caller ignoring ok never gets "".
		return cli, model, false
	}
	return "claude-tmux", model, true
}

// benchedFamilies is the set of families the clihealth store benches now.
func benchedFamilies(projectRoot string) map[string]bool {
	out := map[string]bool{}
	for family := range clihealth.NewStore(projectRoot, nil).Active() {
		out[family] = true
	}
	return out
}

// resolveRouterDispatch resolves the routing advisor's base {cli, model}:
// claude-tmux/opus, then .evolve/profiles/router.json, then RouterPolicy.
func resolveRouterDispatch(evolveDir string, rc policy.RouterPolicy) (cli, model string) {
	cli, model = "claude-tmux", "opus"
	if raw, err := os.ReadFile(filepath.Join(evolveDir, "profiles", "router.json")); err == nil {
		var pj struct {
			CLI              string `json:"cli"`
			ModelTierDefault string `json:"model_tier_default"`
		}
		if json.Unmarshal(raw, &pj) == nil {
			if pj.CLI != "" {
				cli = pj.CLI
			}
			if pj.ModelTierDefault != "" {
				model = pj.ModelTierDefault
			}
		}
	}
	if rc.CLI != "" {
		cli = rc.CLI
	}
	if rc.Model != "" {
		model = rc.Model
	}
	return cli, model
}

// registerBuiltinSpecRunners gives each advisor-selectable kind:llm builtin
// phase that lacks a hand-wired runner a spec runner, so selecting it never
// aborts dispatch. Control phases are skipped; a phase with no persona WARNs.
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

// registrarMinter adapts phaseregistrar.Registrar to core.PhaseMinter, keeping
// core decoupled from phaseregistrar.
type registrarMinter struct{ r phaseregistrar.Registrar }

func (m registrarMinter) Register(cfg phaseconfig.PhaseConfig) (phasespec.PhaseSpec, core.PhaseRunner, error) {
	res, err := m.r.Register(cfg)
	if err != nil {
		return phasespec.PhaseSpec{}, nil, err
	}
	return res.Spec, res.Runner, nil
}

// scopePathResolver maps a scoped task id to its pending inbox record's path,
// or "". Named rather than inline so the root's wiring is testable.
func scopePathResolver(projectRoot, taskID string) string {
	st := inboxmover.ResolveDispatchState(inboxmover.Options{ProjectRoot: projectRoot}, taskID)
	if st.State != inboxmover.StatePending {
		return ""
	}
	return st.Path
}

// failureAdvisorOpts resolves the failure advisor's CLI from its tracked
// profile; an absent or unreadable profile keeps the compiled default.
func failureAdvisorOpts(projectRoot string) []core.FailureAdvisorOption {
	// Pinned: resolvellm's git fallback runs from the process cwd, which can be
	// another tree.
	r, err := resolvellm.Resolve("failure-advisor", resolvellm.Options{ProjectRoot: projectRoot, GitRoot: projectRoot})
	if err != nil || r.CLI == "" {
		return nil
	}
	return []core.FailureAdvisorOption{core.WithFailureAdvisorCLI(r.CLI)}
}

// documentSpecPtr is the registry's document contract, the one resolution the
// build floor and the audit gate share.
func documentSpecPtr(cfg config.RoutingConfig) *config.DeliverableKindSpec {
	spec, ok := cfg.DocumentSpec()
	if !ok {
		return nil
	}
	return &spec
}
