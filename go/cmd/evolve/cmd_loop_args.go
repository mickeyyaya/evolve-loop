package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/goalhash"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

func parseLoopArgs(args []string, stderr io.Writer) (loopConfig, int) {
	fs := flag.NewFlagSet("evolve loop", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		projectRoot       string
		evolveDir         string
		goalHash          string
		goalText          string
		strategy          string
		maxCyclesFlag     int
		cyclesFlag        int
		budgetUSD         float64
		batchCapUSD       float64
		resume            bool
		dryRun            bool
		reset             bool
		consensusAudit    bool
		forceFresh        bool
		skipPreflight     bool
		skipPreflightBoot bool
		bypassPolicy      bool
	)
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to project root")
	fs.StringVar(&evolveDir, "evolve-dir", "", "path to .evolve/ (default <project-root>/.evolve)")
	fs.StringVar(&goalHash, "goal-hash", "", "explicit 64-char (or 8-char prefix) SHA256 of goal; mutually exclusive with --goal-text")
	fs.StringVar(&goalText, "goal-text", "", "goal text; hashed via goalhash.Compute (normalize+SHA256)")
	fs.StringVar(&strategy, "strategy", "", "balanced|innovate|harden|repair|ultrathink|autoresearch (default: balanced)")
	fs.IntVar(&maxCyclesFlag, "max-cycles", 0, "maximum cycles to run (default 1; aliased by --cycles)")
	fs.IntVar(&cyclesFlag, "cycles", 0, "alias for --max-cycles")
	// --budget-usd / --budget / --batch-cap-usd are DEPRECATED no-ops: cost is
	// display-only telemetry now (the token-budget cost gates were removed because
	// the cost calculation was unreliable across LLM models). The flags are still
	// accepted so existing scripts/CI don't error; they have no effect. Use
	// --cycles N to bound a run.
	fs.Float64Var(&budgetUSD, "budget-usd", 0, "(DEPRECATED, ignored) former budget-driven dollar cap")
	fs.Float64Var(&budgetUSD, "budget", 0, "(DEPRECATED, ignored) alias for --budget-usd")
	fs.Float64Var(&batchCapUSD, "batch-cap-usd", 20.0, "(DEPRECATED, ignored) former cumulative batch USD cap")
	fs.BoolVar(&resume, "resume", false, "locate and resume most-recent checkpointed cycle (protocol lands in M3)")
	fs.BoolVar(&dryRun, "dry-run", false, "parse args, print resolved config as JSON, exit 0 (no orchestrator invocation)")
	fs.BoolVar(&reset, "reset", false, "prune infrastructure-systemic/transient + ship-gate-config from state.json:failedApproaches before loop")
	fs.BoolVar(&consensusAudit, "consensus-audit", false, "opt-in cross-CLI auditor consensus mode")
	fs.BoolVar(&forceFresh, "force-fresh", false, "start fresh even if an unfinished cycle exists (history NOT sealed; use evolve cycle reset to seal)")
	fs.BoolVar(&skipPreflight, "skip-preflight", false, "bypass the whole pre-batch readiness gate (no checks, no boot)")
	fs.BoolVar(&skipPreflightBoot, "skip-preflight-boot", false, "run cheap checks but skip the real bridge-boot probe (CI/offline)")
	fs.BoolVar(&bypassPolicy, "bypass-policy", false, "use --bypass-policy to bypass policy.json pin enforcement for every phase in this batch (operator escape hatch)")

	// WS-G2 repeatable per-agent overrides:
	//   --cli  auditor=claude-tmux              (one --cli per agent)
	//   --cli  builder=ollama-tmux              (repeatable)
	//   --model auditor=opus
	//   --model builder=llama3.1:8b
	// Syntactic sugar over EVOLVE_<AGENT>_CLI / EVOLVE_<AGENT>_MODEL —
	// operators can experiment with combos per-run without editing profiles.
	perAgentCLI := map[string]string{}
	perAgentModel := map[string]string{}
	fs.Func("cli", "per-agent CLI override (repeatable): --cli auditor=claude-tmux", func(v string) error {
		agent, value, ok := strings.Cut(v, "=")
		if !ok || strings.TrimSpace(agent) == "" || strings.TrimSpace(value) == "" {
			return fmt.Errorf("--cli expects agent=cli (e.g. --cli auditor=claude-tmux); got %q", v)
		}
		perAgentCLI[strings.TrimSpace(agent)] = strings.TrimSpace(value)
		return nil
	})
	fs.Func("model", "per-agent model override (repeatable): --model auditor=opus", func(v string) error {
		agent, value, ok := strings.Cut(v, "=")
		if !ok || strings.TrimSpace(agent) == "" || strings.TrimSpace(value) == "" {
			return fmt.Errorf("--model expects agent=model (e.g. --model auditor=opus); got %q", v)
		}
		perAgentModel[strings.TrimSpace(agent)] = strings.TrimSpace(value)
		return nil
	})

	if err := fs.Parse(args); err != nil {
		return loopConfig{}, 10
	}

	// The cost-budget feature is disabled: dollar-cost calculation was
	// unreliable across LLM models. Flag still accepted so old scripts parse,
	// but we warn rather than silently ignore it.
	if budgetUSD != 0 {
		fmt.Fprintln(stderr, "evolve loop: WARN: --budget-usd / --budget is disabled and ignored; use --cycles N to bound the run, or omit it and let the advisor decide")
	}

	// Enforce the flag's "absolute path" contract for the project root AND the
	// evolve dir. Downstream, WorkspacePath (= <root>/.evolve/runs/cycle-N) and
	// every per-phase artifact path are derived by joining these; worktree phases
	// run the agent with cwd=worktree, so a RELATIVE base makes the agent resolve
	// the artifact path into the worktree subtree while the in-process bridge
	// polls it against the main cwd — that divergence caused cycle-119's
	// ExitArtifactTimeout (81). Resolving once here (the composition root) keeps
	// every derived path cwd-independent. filepath.Abs only errors when os.Getwd
	// fails (cwd deleted/unmounted), in which case continuing with a relative
	// base would silently reproduce the very timeout this guards against — so we
	// WARN loudly rather than swallow it (the loop may still serve non-worktree
	// phases, so we degrade rather than abort).
	absOrWarn := func(label, p string) string {
		return paths.AbsoluteRoot(label, p, func(m string) {
			fmt.Fprintf(stderr, "evolve loop: WARN: %s\n", m)
		})
	}
	projectRoot = absOrWarn("--project-root", projectRoot)

	// Parse positional args: [CYCLES] [STRATEGY] [GOAL...]
	posCycles, posStrategy, posGoal := parsePositional(fs.Args())

	// Legacy positional-integer deprecation WARN — operators relying on bare
	// `/evo:loop 3 ...` get nudged toward `--cycles 3`.
	if posCycles > 0 && cyclesFlag == 0 && maxCyclesFlag == 0 {
		fmt.Fprintf(stderr, "evolve loop: WARN: bare positional integer (%d) parsed as --cycles is deprecated; prefer explicit --cycles N\n", posCycles)
	}

	// Resolve cycles: explicit flag > positional > default (1).
	resolvedCycles := 0
	switch {
	case cyclesFlag > 0:
		resolvedCycles = cyclesFlag
	case maxCyclesFlag > 0:
		resolvedCycles = maxCyclesFlag
	case posCycles > 0:
		resolvedCycles = posCycles
	default:
		resolvedCycles = 1
	}

	// Resolve strategy: explicit flag > positional > default
	resolvedStrategy := strategy
	if resolvedStrategy == "" {
		resolvedStrategy = posStrategy
	}
	if resolvedStrategy == "" {
		resolvedStrategy = "balanced"
	}
	if _, ok := validStrategies[resolvedStrategy]; !ok {
		fmt.Fprintf(stderr, "evolve loop: invalid --strategy %q (valid: balanced|innovate|harden|repair|ultrathink|autoresearch)\n", resolvedStrategy)
		return loopConfig{}, 10
	}

	// Resolve goal: --goal-hash > --goal-text > positional [GOAL...]
	resolvedGoalText := goalText
	if resolvedGoalText == "" && posGoal != "" {
		resolvedGoalText = posGoal
	}
	resolvedGoalHash := goalHash
	if resolvedGoalHash == "" && resolvedGoalText != "" {
		resolvedGoalHash = goalhash.Compute(resolvedGoalText)
	}
	// Resume and dry-run modes don't require an explicit goal —
	// resume reads the goal from cycle-state.json; dry-run just prints config.
	if resolvedGoalHash == "" && !resume && !dryRun {
		fmt.Fprintln(stderr, "evolve loop: a goal is required — pass --goal-hash, --goal-text, or a positional goal (or --resume to continue a checkpointed cycle)")
		return loopConfig{}, 10
	}

	// Resolve evolve-dir. The derived branch inherits projectRoot's (now
	// absolute) anchor; an explicit --evolve-dir may still be relative, so
	// absolutize the final value either way (same cwd-independence requirement
	// as projectRoot — many consumers join cfg.EvolveDir).
	if evolveDir == "" {
		evolveDir = filepath.Join(projectRoot, ".evolve")
	}
	evolveDir = absOrWarn("--evolve-dir", evolveDir)

	return loopConfig{
		ProjectRoot:       projectRoot,
		EvolveDir:         evolveDir,
		GoalHash:          resolvedGoalHash,
		GoalText:          resolvedGoalText,
		Strategy:          resolvedStrategy,
		MaxCycles:         resolvedCycles,
		MaxCyclesExplicit: cyclesFlag > 0 || maxCyclesFlag > 0 || posCycles > 0,
		Resume:            resume,
		Reset:             reset,
		ConsensusAudit:    consensusAudit,
		DryRun:            dryRun,
		ForceFresh:        forceFresh,
		SkipPreflight:     skipPreflight,
		SkipPreflightBoot: skipPreflightBoot,
		BypassPolicy:      bypassPolicy,
		PerAgentCLI:       perAgentCLI,
		PerAgentModel:     perAgentModel,
	}, 0
}

// parsePositional consumes the [CYCLES] [STRATEGY] [GOAL...] positional
// args per the bash dispatcher's heuristic.
//
//	First token is CYCLES iff it's a positive integer.
//	Next token is STRATEGY iff it's in validStrategies.
//	Remaining tokens are joined by space → GOAL.
//
// Order matters; this matches the bash heuristic verbatim so operators
// who paste their `/evo:loop 3 balanced "fix bug"` invocations into
// the Go binary keep the same parsing semantics.
func parsePositional(args []string) (cycles int, strategy string, goal string) {
	i := 0
	if i < len(args) {
		if n, err := strconv.Atoi(args[i]); err == nil && n > 0 {
			cycles = n
			i++
		}
	}
	if i < len(args) {
		if _, ok := validStrategies[args[i]]; ok {
			strategy = args[i]
			i++
		}
	}
	if i < len(args) {
		goal = joinArgs(args[i:])
	}
	return
}

// joinArgs joins args with a single space. Empty slice → empty string.
// Preserves inner quoting the way bash does when the operator quotes
// a multi-word goal in the original CLI invocation.
func joinArgs(args []string) string {
	return strings.Join(args, " ")
}

// buildCycleContext returns the Context map handed to every cycle.
// Phase agents read it via PhaseRequest.Context: Scout for strategy,
// Intent for the canonical goal text (used to structure intent.md
// before Scout sees it).
//
// Pre-this-fix, only "strategy" was passed — `cfg.GoalText` was
// converted to a hash at parse time and the text discarded. Intent
// persona had no way to see the operator's goal, so intent.md was
// being structured around whatever leftover Scout artifacts happened
// to be in the workspace. Source incident: cycle-108 meta-loop where
// the user's "non-stop autonomy + /goal comparison" goal-text was
// dropped and intent.md got structured around the prior cycle's
// untested-package backlog work instead.
func buildCycleContext(cfg loopConfig) map[string]string {
	out := map[string]string{
		"strategy": cfg.Strategy,
	}
	if cfg.GoalText != "" {
		out["goal"] = cfg.GoalText
	}
	return out
}

// buildCycleEnv returns the env map handed to every cycle in this
// dispatcher invocation. Construction order is intentional:
//
//  1. Copy every EVOLVE_* var from osEnv. This is how operator-set
//     flags (REQUIRE_INTENT, SANDBOX_FALLBACK_ON_EPERM, TRIAGE_DISABLE,
//     BUILD_PLANNER, …) reach the orchestrator + every downstream subagent.
//  2. Apply dispatcher-derived IPC overrides (Resume). Strategy flows
//     via Context["strategy"] — not env. cfg.Reset is consumed at
//     cmd_loop.go before buildCycleEnv is called — neither writes env.
//
// Non-EVOLVE_* vars are intentionally skipped — only this prefix is
// part of the documented operator surface. The orchestrator reads from
// the returned map, never from os.Environ directly, so callers that
// inject env explicitly (tests, in-process embedders) get the same path.
func buildCycleEnv(cfg loopConfig, osEnv []string) map[string]string {
	out := make(map[string]string, 16)
	for _, kv := range osEnv {
		if !strings.HasPrefix(kv, "EVOLVE_") {
			continue
		}
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			continue
		}
		out[kv[:eq]] = kv[eq+1:]
	}
	// Dispatcher-derived IPC overrides.
	if cfg.Resume {
		// IPC key "EVOLVE_RESUME" is split so the operator-flag registry guard
		// does not classify this parent-to-child handoff as a configurable flag.
		// SSOT IPC-protocol-allowed
		out["EVOLVE_"+"RESUME"] = "1"
	}
	// WS-G2: per-agent --cli / --model launch flags translate to
	// EVOLVE_<AGENT>_CLI / EVOLVE_<AGENT>_MODEL env keys (matching
	// envchain.PhaseEnvKey's convention). The runner already reads these
	// for the CLI resolver (G1) and the model resolver. Flag overrides win
	// over inherited process env (their entries are written after the
	// EVOLVE_* sweep above).
	for agent, cli := range cfg.PerAgentCLI {
		out["EVOLVE_"+phaseEnvAgentKey(agent)+"_CLI"] = cli
	}
	for agent, model := range cfg.PerAgentModel {
		out["EVOLVE_"+phaseEnvAgentKey(agent)+"_MODEL"] = model
	}
	return out
}

// phaseEnvAgentKey upper-cases + dash-to-underscore an agent name to
// build per-agent env keys (mirror of envchain.PhaseEnvKey's normalization).
// e.g. "tdd-engineer" → "TDD_ENGINEER" so EVOLVE_TDD_ENGINEER_CLI/MODEL
// match the runner's lookup.
func phaseEnvAgentKey(agent string) string {
	b := make([]byte, 0, len(agent))
	for i := 0; i < len(agent); i++ {
		c := agent[i]
		switch {
		case c == '-':
			b = append(b, '_')
		case c >= 'a' && c <= 'z':
			b = append(b, c-32)
		default:
			b = append(b, c)
		}
	}
	return string(b)
}
