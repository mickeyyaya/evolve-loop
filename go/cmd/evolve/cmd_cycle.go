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

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/observer"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
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
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

// runCycle implements `evolve cycle <subcommand>`. Subcommands: run.
func runCycle(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve cycle: missing subcommand (try: run | reset)")
		return 10
	}
	switch args[0] {
	case "run":
		return runCycleRun(args[1:], stdout, stderr)
	case "reset":
		return runCycleReset(args[1:], stdout, stderr)
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
	fs.BoolVar(&force, "force", false, "seal even if a dispatcher appears to hold the .evolve lock")
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

	// Refuse to seal under a live dispatcher — it holds the .evolve flock
	// (LOCK_NB). Skipped for dry-run (read-only) and when --force is set.
	if !dryRun && !force {
		st := storage.New(evolveDir)
		release, err := st.AcquireLock(context.Background())
		if err != nil {
			fmt.Fprintf(stderr, "evolve cycle reset: .evolve appears locked by a running dispatcher (%v); stop it first or pass --force\n", err)
			return 1
		}
		defer func() { _ = release() }()
	}

	res, err := core.SealCycle(context.Background(), ledger.New(evolveDir), core.SealOptions{
		EvolveDir:   evolveDir,
		ProjectRoot: projectRoot,
		Reason:      reason,
		DryRun:      dryRun,
	})
	if err != nil {
		if errors.Is(err, core.ErrNothingToReset) {
			fmt.Fprintln(stderr, "evolve cycle reset: no in-progress cycle to seal")
			return 1
		}
		fmt.Fprintf(stderr, "evolve cycle reset: %v\n", err)
		return 1
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
		projectRoot string
		goalHash    string
		evolveDir   string
		budgetUSD   float64
		batchCapUSD float64
		simulate    bool
	)
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to the project root (default cwd)")
	fs.StringVar(&goalHash, "goal-hash", "", "8-char SHA256 of the goal (required)")
	fs.StringVar(&evolveDir, "evolve-dir", "", "path to .evolve/ state directory (default <project-root>/.evolve)")
	fs.Float64Var(&budgetUSD, "budget-usd", 999999, "per-cycle USD budget cap")
	fs.Float64Var(&batchCapUSD, "batch-cap-usd", 20.0, "cumulative batch USD cap across cycles")
	fs.BoolVar(&simulate, "simulate", false, "no-LLM walk: every phase returns PASS without calling out (for parity-audit harness)")
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

	var orch *core.Orchestrator
	if simulate {
		orch = wireSimulateOrchestrator(projectRoot, evolveDir)
	} else {
		orch = wireOrchestrator(projectRoot, evolveDir)
	}
	result, err := orch.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: projectRoot,
		GoalHash:    goalHash,
		Budget: core.BudgetEnvelope{
			MaxUSD:      budgetUSD,
			BatchCapUSD: batchCapUSD,
		},
		Env: filterEvolveEnv(os.Environ()),
		Context: map[string]string{
			"commit_message": fmt.Sprintf("evolve-cycle: goal=%s", goalHash),
		},
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
func filterEvolveEnv(environ []string) map[string]string {
	out := map[string]string{}
	for _, kv := range environ {
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			continue
		}
		k := kv[:eq]
		if strings.HasPrefix(k, "EVOLVE_") || strings.HasPrefix(k, "BRIDGE_") {
			out[k] = kv[eq+1:]
		}
	}
	return out
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
		_ = os.Setenv("EVOLVE_MODEL_CATALOG_DIR", evolveDir)
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
	prm := newPromptsLoader(projectRoot)

	runners := map[core.Phase]core.PhaseRunner{
		core.PhaseIntent: intent.New(intent.Config{Bridge: br, Prompts: prm}),
		// Scout + Build are swarm-eligible (ADR-0032): wrapped in the swarmRunner
		// Decorator so EVOLVE_SWARM_STAGE=advisory|enforce dispatches them across N
		// parallel workers (reader fan-out / writer merge-train). Default (stage
		// unset) = byte-identical delegate to the inner runner — zero behavior change.
		core.PhaseScout:        swarmrunner.New(scout.New(scout.Config{Bridge: br, Prompts: prm}), br, swarm.ModeReader),
		core.PhaseTriage:       triage.New(triage.Config{Bridge: br, Prompts: prm}),
		core.PhaseTDD:          tdd.New(tdd.Config{Bridge: br, Prompts: prm}),
		core.PhaseBuildPlanner: buildplanner.New(buildplanner.Config{Bridge: br, Prompts: prm}).BaseRunner(),
		core.PhaseBuild:        swarmrunner.New(build.New(build.Config{Bridge: br, Prompts: prm}), br, swarm.ModeWriter),
		core.PhaseAudit:        audit.NewDefault(br, prm),
		core.PhaseShip:         ship.NewWithDefaultRunner(),
		core.PhaseRetro:        retro.New(retro.Config{Bridge: br, Prompts: prm}),
		// Ship-error recovery phase (Component #8): the advisor's recovery chain
		// routes an unknown/novel ShipError here to diagnose + decide RESHIP /
		// RERUN_PHASE / BLOCK. Optional — never on the mandatory spine.
		core.PhaseDebugger: debugger.New(debugger.Config{Bridge: br, Prompts: prm}),
	}

	// Composition root: the SOLE reader of routing env+config. config.Load
	// maps the central registry + contained env overrides into one immutable
	// RoutingConfig; router.Select picks the brain once. Default
	// dynamic_routing=0 (Stage:Off) ⇒ NewOrchestrator behaves exactly as
	// before. A nil proposer means DynamicLLM degrades to the deterministic
	// StaticPreset (the bridge-backed Proposer is a tracked follow-on).
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
	// consulted at dispatch by the runner.
	if pol, perr := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json")); perr != nil {
		fmt.Fprintf(os.Stderr, "[policy] WARN %v (mandatory merge skipped; fails loudly at dispatch)\n", perr)
	} else {
		cfg.Mandatory = pol.MergeMandatory(cfg.Mandatory)
	}

	// User-defined phases ("Lego" overlays): merge .evolve/phases/<name>/phase.json
	// over the built-in catalog, splice the VALID ones into the routing order +
	// triggers (the floor is enforced here — invalid specs are never routed), and
	// register a spec-driven runner for each so the orchestrator can execute it.
	// No user phases ⇒ all of this is a no-op and behavior is byte-identical.
	builtinCat, _ := phasespec.Load(registryPath)
	userSpecs, discWarns := phasespec.DiscoverUserSpecs(filepath.Join(projectRoot, ".evolve", "phases"))
	catalog, mergeWarns := builtinCat.Merge(userSpecs)
	for _, w := range append(discWarns, mergeWarns...) {
		fmt.Fprintf(os.Stderr, "[phases] WARN %s\n", w)
	}
	for _, w := range phasespec.ApplyUserRouting(&cfg, userSpecs) {
		fmt.Fprintf(os.Stderr, "[phases] WARN %s\n", w)
	}
	// Register a spec-driven runner for each valid user phase. ValidateUserSpec
	// is not called again here — ApplyUserRouting already enforced the floor and
	// emitted warnings for any invalid spec above.
	for _, s := range catalog.UserPhases() {
		if len(phasespec.ValidateUserSpec(s)) > 0 {
			continue // ApplyUserRouting already warned + skipped it; no dead runner
		}
		if _, exists := runners[core.Phase(s.Name)]; !exists {
			runners[core.Phase(s.Name)] = specrunner.New(s, specrunner.Config{Bridge: br, Prompts: prm})
		}
	}
	// DynamicLLM brain: a bridge-backed proposer. Select uses it only when
	// routing_mode=llm; otherwise it falls back to the deterministic
	// StaticPreset. Either way the kernel clamp in router.Route is the floor.
	advisor := core.NewPhaseAdvisor(br)
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
	}
	// Cycle-122 Fix 3 / ADR-0030: auto-spawn the per-phase observer
	// goroutine unless explicitly opted out via EVOLVE_OBSERVER_AUTOSPAWN=0.
	// Restores the pre-v12 bash-dispatcher behavior the Go port silently
	// dropped. The default StallS=600s matches the bridge's coarse
	// artifact-timeout; an operator who wants faster detection can set
	// EVOLVE_OBSERVER_STALL_S=90 (or similar).
	if os.Getenv("EVOLVE_OBSERVER_AUTOSPAWN") != "0" {
		opts = append(opts, core.WithObserver(observer.NewCoreAdapter()))
	}
	// Cycle-start live model-catalog refresh (TTL=1 day, gated + best-effort
	// inside the closure). Opt out with EVOLVE_MODELCATALOG_AUTOREFRESH=0.
	opts = append(opts, core.WithCatalogRefresher(makeCatalogRefresher(projectRoot, evolveDir)))

	return orchDeps{
		Storage:      st,
		Ledger:       ld,
		Orchestrator: core.NewOrchestrator(st, ld, runners, opts...),
	}
}
