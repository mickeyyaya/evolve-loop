// `evolve cycle run` drives one cycle through the orchestrator. Wires
// storage + ledger adapters with all 8 phase runners (intent through
// retro). Subcommand surface stays small; the orchestrator owns the
// phase sequencing.
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
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/observer"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
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
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	"github.com/mickeyyaya/evolve-loop/go/internal/topngate"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

// runCycle implements `evolve cycle <subcommand>`. Subcommands: run | reset | timing.
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
	default:
		fmt.Fprintf(stderr, "evolve cycle: unknown subcommand %q\n", args[0])
		return 10
	}
}

// runCycleReset seals an unfinished cycle: it archives the workspace +
// cycle-state snapshot + a manifest (history preserved, never deleted),
// appends an auditable ledger entry, advances lastCycleNumber so the number
// is never reused, and clears cycle-state.json. The complement of
// `evolve loop --resume`. See core.SealCycle.
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
	// Absolutize the root so SealCycle's "workspace inside projectRoot" check
	// compares absolute-to-absolute. A relative root here refused to seal
	// cycle-120 (whose workspace_path was absolute, written by the fixed loop).
	projectRoot = paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve cycle reset: WARN: %s\n", m)
	})
	if evolveDir == "" {
		evolveDir = filepath.Join(projectRoot, ".evolve")
	}

	// The liveness fence lives in SealCycle — it reads the per-run .lease
	// heartbeat (the SSOT for "is the owner alive?"). The OLD `.evolve/.lock`
	// pre-check here was a false negative that caused the cycle-395 race: the
	// dispatcher's lock is per-CYCLE (released between cycles), so a sibling
	// reset acquired it in the gap and sealed a RUNNING loop. We pass Force
	// through and let the heartbeat-backed fence decide.
	res, err := core.SealCycle(context.Background(), ledger.New(evolveDir), core.SealOptions{
		EvolveDir:   evolveDir,
		ProjectRoot: projectRoot,
		Reason:      reason,
		DryRun:      dryRun,
		Force:       force,
		// PID-aware liveness (cycle-554): a crashed owner whose heartbeat has not
		// yet aged past the TTL (the 2-6min post-crash window) is no longer "live",
		// so a plain `evolve cycle reset` seals it WITHOUT --force. A genuinely
		// running owner (alive pid) still refuses. Same probe boot recovery uses.
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
	// The cost-budget flags (--budget-usd/--budget/--batch-cap-usd) are removed
	// from the parameter surface; strip any legacy occurrence (with a WARN) before
	// parse so old scripts don't trip flag.Parse's "not defined" error.
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
	// Absolutize before any path is derived from projectRoot — a relative root
	// (default ".") makes worktree-phase artifact paths diverge across the
	// agent's worktree cwd and the in-process bridge's main cwd (cycle-119).
	projectRoot = paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve cycle run: WARN: %s\n", m)
	})
	if evolveDir == "" {
		evolveDir = filepath.Join(projectRoot, ".evolve")
	}

	if !simulate {
		// Crash-recovery GC before the cycle: reap dead-owner tmux sessions left
		// by a prior crashed run (see gcOrphanSessions). Skipped in --simulate —
		// the no-LLM parity walk launches no sessions.
		gcOrphanSessions("cycle-start", stderr)
	}

	var orch *core.Orchestrator
	if simulate {
		orch = wireSimulateOrchestrator(projectRoot, evolveDir)
	} else {
		orch = wireOrchestrator(projectRoot, evolveDir)
	}
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
		fmt.Fprintf(stderr, "evolve cycle run: %v\n", err)
		return 1
	}
	buf, _ := json.MarshalIndent(result, "", "  ")
	fmt.Fprintln(stdout, string(buf))
	if result.FinalVerdict == core.VerdictFAIL {
		return 2
	}
	return 0
}

// filterEvolveEnv extracts the EVOLVE_* and BRIDGE_* slice of the
// process environment into a flat map. Phases consult these for CLI
// selection (EVOLVE_CLI), model overrides (EVOLVE_*_MODEL), and ship
// behaviour (EVOLVE_SHIP_SCRIPT). BRIDGE_TESTING / BRIDGE_*_BINARY are
// also propagated so test invocations can swap CLI binaries.
// filterEvolveEnv forwards to cmdutil.FilterEvolveEnv — the implementation now
// lives in the cmd/evolve/cmdutil leaf so the decomposed internal/cli/* groups
// share ONE definition. Thin forwarder kept so this file's callers are unchanged.
func filterEvolveEnv(environ []string) map[string]string {
	return cmdutil.FilterEvolveEnv(environ)
}

// wireOrchestrator returns an orchestrator wired with production
// adapters: filesystem-backed storage + ledger, default bridge, all
// 8 phase runners. Extracted for cmd_loop reuse.
func wireOrchestrator(projectRoot, evolveDir string) *core.Orchestrator {
	d := wireOrchestratorDeps(projectRoot, evolveDir)
	return d.Orchestrator
}

// orchDeps is the wired bundle when callers need access to the storage
// and ledger handles in addition to the orchestrator (cmd_loop uses
// the ledger handle for post-cycle verification).
type orchDeps struct {
	Storage      core.Storage
	Ledger       core.Ledger
	Orchestrator *core.Orchestrator
}

// wireOrchestratorDeps mirrors wireOrchestrator but returns the
// underlying storage + ledger so callers can run cross-cutting
// queries (verify the ledger, read state.json) without re-instantiating
// the adapters and risking divergence in the evolveDir resolution.
func wireOrchestratorDeps(projectRoot, evolveDir string) orchDeps {
	// Pin the model-catalog dir to the SAME .evolve the cycle-start refresher
	// writes, so the in-process LoadManifest overlay reads the right file
	// regardless of EVOLVE_PROJECT_ROOT (which the loop resolves from a flag,
	// not necessarily the env). Set unconditionally — evolveDir is authoritative.
	if evolveDir != "" {
		d := evolveDir
		bridge.SetModelCatalogDirFn(func() string { return d })
	}
	// Ledger and storage are created first so the bridge adapter can wire its
	// stop-review callback to append kind=stop_review entries (ADR-0026 Stage 1 #5).
	st := storage.New(evolveDir)
	ld := ledger.New(evolveDir)

	br := bridge.NewDefault(projectRoot)
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

	// Composition root: the SOLE reader of routing env+config. config.Load
	// maps the central registry + contained env overrides into one immutable
	// RoutingConfig; router.Select picks the brain once. With
	// dynamic_routing=0 (Stage:Off, the escape hatch; advisory is the
	// default since 2026-06-06) NewOrchestrator behaves exactly as before. A nil proposer means DynamicLLM degrades to the deterministic
	// StaticPreset (the bridge-backed Proposer is a tracked follow-on).
	// Loaded BEFORE the runners map so cfg.PhaseIO can thread into the
	// build/scout/triage reconcile rung (ADR-0050 §3.10 Slice 1).
	registryPath := filepath.Join(projectRoot, "docs", "architecture", "phase-registry.json")
	cfg, warnings := config.Load(registryPath, filterEvolveEnv(os.Environ()))
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "[config] WARN %s: %s\n", w.Code, w.Message)
	}

	// User policy (.evolve/policy.json): merge mandatory_phases into the routing
	// spine so the advisor can never drop a user-declared mandatory phase. This
	// is ADDITIVE — policy can only ADD mandatory phases; the non-configurable
	// integrity floor (ship ⇒ build ∧ audit, enforced in config.Load) keeps the
	// core spine regardless. A malformed policy is WARNed here (this construction
	// path returns no error) and hard-fails loudly at the first phase dispatch,
	// where the runner re-loads it for pins. Per-phase CLI/model pins are
	// consulted at dispatch by the runner. Loaded BEFORE the runners map so
	// swarm config (policy.SwarmConfig) can be threaded into swarmrunner.New.
	var shipFloor []string // WS4: nil ⇒ orchestrator uses router.DefaultShipFloor
	pol, policyErr := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if policyErr != nil {
		fmt.Fprintf(os.Stderr, "[policy] WARN %v (mandatory merge skipped; fails loudly at dispatch)\n", policyErr)
		pol = policy.Policy{}
	} else {
		cfg.Mandatory = pol.MergeMandatory(cfg.Mandatory)
		// WS4: a user-configured ship_floor (e.g. ["audit"] for audit-only) drives
		// the integrity-floor clamp; absent ⇒ leave nil so the safe default stands.
		if floor, overridden := pol.FloorPhases(); overridden {
			shipFloor = floor
		}
		// Failure floor (Phase 4a): policy.json:failure_floor is the one
		// surface for the audit-FAIL learning route; shared fold with
		// router.PolicyForProject so per-phase consumers see the same route.
		cfg.AuditFailRoutesTo = router.FailureRouteFromPolicy(pol)
	}
	swCfg := swarmrunner.Config{Stage: pol.SwarmConfig().Stage, PortBase: pol.SwarmConfig().PortBase, WorktreeBase: pol.WorktreeBase()}
	gatesCfg := pol.GatesConfig()
	routerCfg := pol.RouterConfig()
	recoveryCfg := pol.RecoveryConfig()
	cfg.ContractGate = parseGateStage(gatesCfg.ContractGate)
	cfg.EvalGate = parseGateStage(gatesCfg.EvalGate)
	cfg.TriageCapGate = parseGateStage(gatesCfg.TriageCapGate)
	cfg.TopNGate = parseGateStage(gatesCfg.TopNGate)
	cfg.ReviewGate = parseGateStage(gatesCfg.ReviewGate)
	cfg.PhaseRecovery = parseGateStage(recoveryCfg.PhaseRecovery)
	cfg.SpineFloor = parseGateStage(recoveryCfg.SpineFloor)
	cfg.RouterReplan = parseRouterStage(routerCfg.RouterReplan)
	peCfg := pol.ParallelEvaluateConfig()
	cfg.ParallelEvaluate = parseRouterStage(peCfg.Stage)
	cfg.ParallelEvaluateConcurrency = peCfg.Concurrency
	cfg.RoutingJudge = routerCfg.RoutingJudge
	cfg.ReconDigest = routerCfg.ReconDigest
	cfg.RePlanMaxDepth = routerCfg.ReplanDepth
	// Resolved once here so all phase constructors below share the same value.
	// Avoids a second pol.WorkflowConfig() call at line ~538.
	wfCfg := pol.WorkflowConfig()

	runners := map[core.Phase]core.PhaseRunner{
		core.PhaseIntent: intent.New(intent.Config{Bridge: br, Prompts: prm, CompactPrompts: cfg.CompactPrompts}),
		// Scout + Build are swarm-eligible (ADR-0032): wrapped in the swarmRunner
		// Decorator so stage=advisory|enforce (policy.json "swarm.stage") dispatches
		// them across N parallel workers (reader fan-out / writer merge-train).
		// Default (stage absent/shadow) = byte-identical delegate — zero behavior change.
		// PhaseIO threads cfg.PhaseIO into the reconcile rung (3.10 Slice 1); StageOff
		// (the shipping default) keeps these byte-identical.
		core.PhaseScout:        swarmrunner.New(scout.New(scout.Config{Bridge: br, Prompts: prm, PhaseIO: cfg.PhaseIO, CompactPrompts: cfg.CompactPrompts}), br, swarm.ModeReader, swCfg),
		core.PhaseTriage:       triage.New(triage.Config{Bridge: br, Prompts: prm, PhaseIO: cfg.PhaseIO, CompactPrompts: cfg.CompactPrompts}),
		core.PhaseTDD:          tdd.New(tdd.Config{Bridge: br, Prompts: prm, CompactPrompts: cfg.CompactPrompts}),
		core.PhaseBuildPlanner: buildplanner.New(buildplanner.Config{Bridge: br, Prompts: prm}).BaseRunner(),
		core.PhaseBuild:        swarmrunner.New(build.New(build.Config{Bridge: br, Prompts: prm, PhaseIO: cfg.PhaseIO, CompactPrompts: cfg.CompactPrompts}), br, swarm.ModeWriter, swCfg),
		core.PhaseAudit:        audit.NewDefaultWithStageCompact(br, prm, cfg.PhaseIO, cfg.CompactPrompts),
		core.PhaseShip:         ship.NewWithDefaultRunnerStage(cfg.PhaseIO),
		core.PhaseRetro:        retro.New(retro.Config{Bridge: br, Prompts: prm, Model: "auto", CompactPrompts: cfg.CompactPrompts}),
		// Ship-error recovery phase (Component #8): the advisor's recovery chain
		// routes an unknown/novel ShipError here to diagnose + decide RESHIP /
		// RERUN_PHASE / BLOCK. Optional — never on the mandatory spine.
		core.PhaseDebugger: debugger.New(debugger.Config{Bridge: br, Prompts: prm, CompactPrompts: cfg.CompactPrompts}),
	}

	// User-defined phases ("Lego" overlays): merge .evolve/phases/<name>/phase.json
	// over the built-in catalog, splice the VALID ones into the routing order +
	// triggers (the floor is enforced here — invalid specs are never routed), and
	// register a spec-driven runner for each so the orchestrator can execute it.
	// No user phases ⇒ all of this is a no-op and behavior is byte-identical.
	builtinCat, builtinErr := phasespec.Load(registryPath)
	if builtinErr != nil {
		// Non-fatal (matches config.Load's tolerant posture above), but no longer
		// silent: a missing/malformed registry means registerBuiltinSpecRunners
		// wires nothing, so a selectable phase could later abort at dispatch —
		// surface the cause here rather than leave it undiagnosable.
		fmt.Fprintf(os.Stderr, "[phases] WARN builtin registry load failed (%v); builtin spec-runners not registered\n", builtinErr)
	}
	userSpecs, _, discWarns := phasespec.DiscoverUserSpecsFromRoots(phaseRoots(projectRoot))
	catalog, mergeWarns := builtinCat.Merge(userSpecs)
	for _, w := range append(discWarns, mergeWarns...) {
		fmt.Fprintf(os.Stderr, "[phases] WARN %s\n", w)
	}
	// Make the bridge catalog-aware so user/minted phases get their spec-derived
	// Deliverable Contract block + exact-path footer injected (WS-A, ADR-0034).
	br.SetContractResolver(phasecontract.NewCatalogResolver(catalog.Get))
	// ADR-0050 §3.8b: at EVOLVE_PHASE_IO>=advisory the injected contract block
	// instructs build/scout/triage to self-report failure via a structured
	// sentinel; default (off) leaves the dispatched prompt byte-identical.
	br.SetPhaseIOStage(cfg.PhaseIO)
	br.SetRecoveryStage(cfg.PhaseRecovery.String())
	for _, w := range phasespec.ApplyUserRouting(&cfg, userSpecs, builtinCat) {
		fmt.Fprintf(os.Stderr, "[phases] WARN %s\n", w)
	}
	// Register a spec-driven runner for each valid user phase. This MUST use the
	// same catalog-aware validator as ApplyUserRouting above (line ~399): the
	// bare ValidateUserSpec re-imposes the two-tier single-word naming floor that
	// ValidateUserSpecWithCatalog exempts for optional built-ins (e.g. "memo"),
	// so routing would nominate the phase while dispatch silently dropped it —
	// the cycle-563 memo-dispatch bug. Keep these two call sites in lockstep.
	for _, s := range catalog.UserPhases() {
		if len(phasespec.ValidateUserSpecWithCatalog(s, builtinCat)) > 0 {
			continue // ApplyUserRouting already warned + skipped it; no dead runner
		}
		if _, exists := runners[core.Phase(s.Name)]; !exists {
			runners[core.Phase(s.Name)] = specrunner.New(s, specrunner.Config{Bridge: br, Prompts: prm})
		}
	}
	// Spec-runner fallback for BUILTIN registry phases the advisor can SELECT
	// (see registerBuiltinSpecRunners) — makes the invariant "every
	// advisor-selectable phase is dispatchable" hold.
	registerBuiltinSpecRunners(runners, builtinCat, prm, br, os.Stderr)
	// DynamicLLM brain: the routing advisor, defined like every phase agent —
	// persona (agents/evolve-router.md) + profile (router.json) + artifact. Its
	// {cli, model} resolve from the profile + EVOLVE_ROUTER_CLI/_MODEL env (the
	// same precedence phases use), so the brain is configurable to any LLM CLI
	// (claude/opus default, or codex/gpt-5.5-high, agy/gemini, …). Composing the
	// cycle + minting phases is deep-reasoning work, hence the opus/deep default.
	// Select consults it only at routing_mode=llm; the kernel clamp is the floor.
	// The advisor's PRIMARY model is the plan/re-plan (deep) dispatch — the
	// confidence-critical decision. WS6-S1: routed through the per-decision-type
	// resolver so policy can override it; no-op vs the prior resolveRouterDispatch
	// when unset. WS6-S2: if the resolved family is benched
	// (the cli-health circuit breaker), fall back to the healthy claude family;
	// when even that is benched the advisor keeps the benched dispatch and degrades
	// to the static spine via its existing fail-safe (clihealth IS the breaker).
	advCLI, advModel, advHealthy := resolveRouterDispatchHealthy(evolveDir, decisionPlan, benchedFamilies(projectRoot), routerCfg)
	if !advHealthy {
		// advCLI/advModel are the base (benched) dispatch — usable; the advisor's
		// dispatch will fail on the benched family and degrade to the static spine.
		fmt.Fprintf(os.Stderr, "[router] WARN router family and the claude fallback are both benched — advisor will degrade to the static spine\n")
	}
	var advPersona string
	if rp, perr := prm.Agent("evolve-router"); perr == nil {
		advPersona = rp.Body
	} else {
		fmt.Fprintf(os.Stderr, "[router] WARN persona evolve-router.md not loaded (%v); advisor uses legacy inline framing\n", perr)
	}
	advisor := core.NewPhaseAdvisor(br,
		core.WithProposerCLI(advCLI),
		core.WithProposerModel(advModel),
		core.WithPersona(advPersona),
		core.WithDepthCheck(core.AdvisorDepthExceeded),
	)
	strategy := router.Select(cfg, advisor)
	// The same advisor also produces the upfront whole-cycle plan the integrity
	// floor clamps (ADR-0024 §2). Wire it unconditionally: the orchestrator is the
	// single gate — it consults the planner only at Stage>=Advisory AND
	// Mode==DynamicLLM, so in static mode or below Advisory it is never called
	// (no LLM cost), and the kernel falls back to the configurable spine.
	opts := []core.Option{
		core.WithRouting(cfg, strategy),
		core.WithCatalog(catalog),
		core.WithPlanner(advisor),
		// ADR-0044 Slice-6 post-soak step (R8.1): the LLM failure-advisor
		// tail. Wired unconditionally — the hook is enforce-gated
		// (cfg.PhaseRecovery) and best-effort, so below enforce it never
		// dispatches; at enforce it turns one unclassified fatal pane into a
		// validated promotion (each promotion saves ~20 min of maxExtends
		// burn on every future occurrence).
		core.WithFailureAdviser(core.NewFailureAdvisor(br)),
		// R9.1 triage-capacity: record shipped cycles' committed-floor counts
		// into the rolling throughput window (state.json:triageThroughput) —
		// the observed-capacity signal the R9.2 clamp bounds triage with.
		core.WithThroughputRecorder(triagecap.Recorder(projectRoot)),
		// Mint advisor-proposed phases (Steps 11/12): persist their dispatch
		// profile + spec under .evolve so the unchanged runner resolves them
		// from disk, then dispatch by name like a built-in. Only active when the
		// advisor drives (Stage>=Advisory) and a plan carries MintPhases.
		core.WithRegistrar(registrarMinter{r: phaseregistrar.Registrar{
			Bridge:      br,
			Prompts:     prm,
			ProfilesDir: filepath.Join(evolveDir, "profiles"),
			PhasesDir:   filepath.Join(evolveDir, "phases"),
		}}),
	}
	// Cycle-122 Fix 3 / ADR-0030: auto-spawn the per-phase observer
	// goroutine unless explicitly disabled in policy.json.
	// Restores the pre-v12 bash-dispatcher behavior the Go port silently
	// dropped. The default StallS=600s matches the bridge's coarse
	// artifact-timeout.
	observerCfg := pol.ObserverConfig()
	if *observerCfg.Autospawn {
		ca := observer.NewCoreAdapter(observerCfg)
		ca.RecoveryStage = cfg.PhaseRecovery.String()
		opts = append(opts, core.WithObserver(ca))
	}
	// Structural eval gates (internal/evalgate): Gate A (scout eval-file
	// materialization) + Gate B (tdd predicate-quality), mounted at the
	// per-phase DeliverableReviewer seam. Default enforce (config.defaults);
	// policy.gates.eval_gate=off keeps the noopReviewer default.
	// The gates fail open on any ambiguity, so enforce never false-blocks.
	// Compose the structural eval gates with the deliverable-contract gate
	// (internal/deliverable, ADR-0034) behind ONE reviewer via ChainReviewers —
	// WithReviewer sets a single reviewer, so both gates must be chained at the
	// same seam. Each is gated independently (default enforce); both fail open on
	// ambiguity. With both off, no reviewer is wired (noopReviewer; byte-identical).
	var reviewers []core.DeliverableReviewer
	if cfg.EvalGate != config.StageOff {
		reviewers = append(reviewers, evalgate.NewReviewer(cfg.EvalGate))
	}
	if cfg.ContractGate != config.StageOff {
		// Catalog-aware so user/minted phases get spec-derived contracts (WS-A):
		// the host gate enforces the SAME well-formedness the agent's
		// `evolve phase verify` self-check derives from the phase.json. The
		// report-size gate (cycle-565 S1) rides the same reviewer as its own
		// dial: default shadow (observe-only) so it is byte-identical until an
		// operator promotes gates.report_size_gate to enforce.
		reviewers = append(reviewers, deliverable.NewReviewerWithCatalogStageReportSize(
			cfg.ContractGate, catalog, cfg.PhaseIO,
			parseGateStage(gatesCfg.ReportSizeGate), pol.ReportBudgetConfig().HandoffTokens))
	}
	if cfg.TriageCapGate != config.StageOff {
		// R9.2 triage capacity clamp (internal/triagecap): committed coverage
		// floors above ceil(1.25·K observed throughput) reject the triage
		// deliverable into the correction ladder with a cap directive.
		// Chained AFTER the contract gate: well-formedness first, capacity
		// second. Fails open on ambiguity.
		reviewers = append(reviewers, triagecap.NewReviewer(cfg.TriageCapGate))
	}
	if cfg.TopNGate != config.StageOff {
		// build->audit task-binding clamp (internal/topngate): a build report
		// whose ## Task: slug falls outside triage ## top_n is a CERTAIN
		// wrong-task build; enforce aborts it before audit/ship spend (inbox
		// builder-task-binding-topn-gate, 8th recurrence). Chained after the
		// contract gate: well-formedness first, task-identity binding second.
		// Fails open on ambiguity (missing report, empty top_n).
		reviewers = append(reviewers, topngate.NewReviewer(cfg.TopNGate))
	}
	if len(reviewers) > 0 {
		opts = append(opts, core.WithReviewer(core.ChainReviewers(reviewers...)))
	}
	if cfg.ContractGate != config.StageOff {
		// ADR-0045 I2: the breaker-neutral re-check the salvage rung verifies
		// relocations with — same catalog-aware resolution as the gate, so the
		// rung and the gate can never disagree about "well-formed". Wired at
		// every contract-gate stage (shadow needs it for would-salvage soak
		// telemetry); execution stays gated on EVOLVE_PHASE_RECOVERY=enforce.
		opts = append(opts, core.WithContractVerifier(deliverable.NewVerifierWithCatalogStage(catalog, cfg.PhaseIO)))
	}
	// Cycle-start live model-catalog refresh (TTL=1 day, gated + best-effort
	// inside the closure). Opt out via policy.json "catalog":{"auto_refresh":false}.
	opts = append(opts, core.WithCatalogRefresher(makeCatalogRefresher(projectRoot, evolveDir, *pol.CatalogConfig().AutoRefresh)))

	// Runtime operator-directives provider: the ONLY place that resolves directives
	// config (home dir + runscope lane + file paths). The orchestrator stays
	// config-agnostic and just consumes the snapshot each cycle.
	opts = append(opts, core.WithDirectivesProvider(makeDirectivesProvider(projectRoot)))

	// WS2 knowledge-base recall: wire the lessons corpus so the advisor plans
	// with recall memory (prior failures + lessons). Lookup is best-effort and
	// only consulted at plan time; an absent corpus is a no-op.
	opts = append(opts, core.WithKB(research.NewFileKB(kbRootsAbs(projectRoot))))

	// WS4 configurable integrity floor: pass the user-resolved ship_floor (nil ⇒
	// the orchestrator's safe default). Empty is ignored by WithShipFloor.
	opts = append(opts, core.WithShipFloor(shipFloor))
	opts = append(opts, core.WithRetryConfig(pol.RetryConfig()))
	opts = append(opts, core.WithWorkflowConfig(wfCfg))
	opts = append(opts, core.WithChronicleConfig(pol.ChronicleConfig()))
	opts = append(opts, core.WithWorktreeBase(pol.WorktreeBase()))

	// RUNG 0 trivial-rebase composition-verdict fast path (cycle-786/801 built
	// the pieces, cycle-804 wires them): bind the snapshot / gate-runner /
	// verdict-writer closures so recoverFromShipError's clean fleet-rebase
	// branch carries the audit verdict forward instead of always re-auditing.
	// All fail-closed — see cmd_composition_wiring.go.
	opts = append(opts, compositionOptions()...)

	return orchDeps{
		Storage:      st,
		Ledger:       ld,
		Orchestrator: core.NewOrchestrator(st, ld, runners, opts...),
	}
}

// kbRootsAbs resolves the KB search roots (relative by default,
// e.g. ".evolve/instincts/lessons/") against the project root so the KB reads
// the right corpus regardless of the process cwd. Absolute roots pass through.
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

// cycleContext builds the CycleRequest.Context seed: the commit message always,
// plus Context["goal"] = the human-readable goal text when one was supplied. The
// "goal" key is the convention the `evolve loop` dispatcher uses and that Scout +
// the routing advisor read (Context["strategy"] is the distinct strategy MODE, not
// the goal). Omitting --goal keeps the prior behavior (no goal key).
func cycleContext(goalHash, goalText string) map[string]string {
	ctx := map[string]string{
		"commit_message": fmt.Sprintf("evolve-cycle: goal=%s", goalHash),
	}
	if goalText != "" {
		ctx["goal"] = goalText
	}
	return ctx
}

// resolveRouterDispatch resolves the routing advisor's {cli, model} the same way
// a phase resolves its capability: profile (.evolve/profiles/router.json) defaults,
// overridden by the per-agent env (EVOLVE_ROUTER_CLI / EVOLVE_ROUTER_MODEL). This
// makes the brain configurable to any LLM CLI (e.g. codex-tmux + deep→gpt-5.5).
// Fallback is opus on claude-tmux (deep reasoning for composition/minting).
// routerDecisionType selects which advisor decision a dispatch is resolved for
// (ADR-0052 WS6-S1). The confidence-critical whole-cycle decisions (plan,
// re-plan) want the DEEP tier; the lightweight off-critical-path ones (the
// reactive propose, the route-quality judge) can use the FAST tier (D2).
type routerDecisionType int

const (
	decisionPlan    routerDecisionType = iota // initial whole-cycle plan (deep)
	decisionRePlan                            // post-scout re-plan (deep)
	decisionPropose                           // reactive per-transition tweak (fast)
	decisionJudge                             // route-quality judge (fast)
)

// resolveRouterDispatchFor resolves the (cli, model) for a SPECIFIC advisor
// decision type (ADR-0052 WS6-S1, optional multi-model). It starts from the
// single base dispatch (resolveRouterDispatch) and applies a per-type model
// override from RouterPolicy. With no override set it returns the base value for
// every type. The CLI is unchanged across types; only the model tier differs.
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

// resolveRouterDispatchHealthy resolves the per-decision dispatch and, if the
// chosen CLI's family is currently benched (the cli-health circuit breaker —
// repeated failures bench a family), falls back to the universal claude family
// (the no-agy-fallback rule: claude is the universal fallback). If the claude
// fallback is ALSO benched it returns ok=false and the caller degrades to the
// static spine — the advisor's existing fail-safe, so no separate breaker is
// minted (clihealth IS the breaker; ADR-0052 WS6-S2). benched is the set of
// benched family names (clihealth.Store.Active values' Family).
func resolveRouterDispatchHealthy(evolveDir string, dt routerDecisionType, benched map[string]bool, rc policy.RouterPolicy) (cli, model string, ok bool) {
	cli, model = resolveRouterDispatchFor(evolveDir, dt, rc)
	if !benched[llmroute.Family(cli)] {
		return cli, model, true // primary family healthy
	}
	if benched["claude"] {
		// Primary AND the universal claude fallback are benched. Return the base
		// (benched) dispatch with ok=false: the caller dispatches it and the
		// advisor degrades to the static spine via its existing fail-safe. We
		// return a USABLE dispatch (never empty strings) so a caller that ignores
		// ok still gets a valid CLI rather than a silent misconfiguration.
		return cli, model, false
	}
	return "claude-tmux", model, true // fall back to the healthy claude family
}

// benchedFamilies returns the set of CLI families currently benched by the
// cli-health store (the circuit breaker — repeated transient failures bench a
// family), keyed by family name. An empty/unreadable store ⇒ empty set (no
// fallback needed). Reuses clihealth.Store (the SAME store the advisor prompt's
// bench context reads), never a parallel breaker.
func benchedFamilies(projectRoot string) map[string]bool {
	out := map[string]bool{}
	for family := range clihealth.NewStore(projectRoot, nil).Active() {
		out[family] = true // Active() is keyed by family name
	}
	return out
}

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

func parseGateStage(stage string) config.Stage {
	switch strings.TrimSpace(stage) {
	case "shadow":
		return config.StageShadow
	case "enforce":
		return config.StageEnforce
	default:
		return config.StageOff
	}
}

func parseRouterStage(stage string) config.Stage {
	switch strings.TrimSpace(stage) {
	case "shadow":
		return config.StageShadow
	case "advisory":
		return config.StageAdvisory
	case "enforce":
		return config.StageEnforce
	default:
		return config.StageOff
	}
}

// registerBuiltinSpecRunners wires a spec-driven runner for every builtin
// registry phase the advisor can SELECT (WS3 catalog cards = non-Control
// archetypes) that is declared kind:llm but is absent from the hand-wired
// runners map (e.g. tester, an advisor-selectable architecture-design). Without
// this such a phase is catalog-visible — the advisor can select it — yet has no
// runner, so dispatch would abort (ErrPhaseInvalid). Mutates runners in place.
//
// Control-archetype phases (memo/retrospective) are kernel-dispatched under
// their canonical names and are NOT advisor-selectable, so they are excluded (no
// dead entries). Guarded on persona existence: a kind:llm phase whose
// agents/<name>.md is missing (e.g. plan-review today) is skipped + WARNed to
// `warn` rather than wired to a runner it cannot execute.
func registerBuiltinSpecRunners(runners map[core.Phase]core.PhaseRunner, builtinCat phasespec.Catalog, prm *prompts.Loader, br core.Bridge, warn io.Writer) {
	for _, s := range builtinCat.All() {
		if s.KindOrDefault() != "llm" || s.RoleOrDefault() == phasespec.RoleControl {
			continue
		}
		if _, exists := runners[core.Phase(s.Name)]; exists {
			continue
		}
		if _, err := prm.Agent(s.AgentName()); err != nil {
			fmt.Fprintf(warn, "[phases] WARN selectable spec phase %q (kind:llm) has no runner and no persona %s.md — not dispatchable until a persona is added\n", s.Name, s.AgentName())
			continue
		}
		runners[core.Phase(s.Name)] = specrunner.New(s, specrunner.Config{Bridge: br, Prompts: prm})
	}
}

// registrarMinter adapts the concrete phaseregistrar.Registrar to the narrow
// core.PhaseMinter port (which returns spec + runner separately) so core stays
// decoupled from phaseregistrar/specrunner.
type registrarMinter struct{ r phaseregistrar.Registrar }

func (m registrarMinter) Register(cfg phaseconfig.PhaseConfig) (phasespec.PhaseSpec, core.PhaseRunner, error) {
	res, err := m.r.Register(cfg)
	if err != nil {
		return phasespec.PhaseSpec{}, nil, err
	}
	return res.Spec, res.Runner, nil
}
