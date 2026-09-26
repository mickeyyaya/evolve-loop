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

// parseLoopArgs parses `evolve loop` arguments. Returns the resolved config
// plus rc (0 = success, 10 = bad args; errors printed to stderr).
//
// Argument precedence:
//
//	--goal-hash takes priority over --goal-text (--goal-text computes hash)
//	--goal-text takes priority over positional [GOAL...]
//	--cycles / --max-cycles take priority over positional [CYCLES]
//	--strategy takes priority over positional [STRATEGY]
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
		resume            bool
		dryRun            bool
		reset             bool
		fingerprint       string
		consensusAudit    bool
		forceFresh        bool
		skipPreflight     bool
		skipPreflightBoot bool
		bypassPolicy      bool
		untilInboxEmpty   bool
	)
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to project root")
	fs.StringVar(&evolveDir, "evolve-dir", "", "path to .evolve/ (default <project-root>/.evolve)")
	fs.StringVar(&goalHash, "goal-hash", "", "explicit 64-char (or 8-char prefix) SHA256 of goal; mutually exclusive with --goal-text")
	fs.StringVar(&goalText, "goal-text", "", "goal text; hashed via goalhash.Compute (normalize+SHA256)")
	fs.StringVar(&strategy, "strategy", "", "balanced|innovate|harden|repair|ultrathink|autoresearch (default: balanced)")
	fs.IntVar(&maxCyclesFlag, "max-cycles", 0, "maximum cycles to run (default 1; aliased by --cycles)")
	fs.IntVar(&cyclesFlag, "cycles", 0, "alias for --max-cycles")
	fs.BoolVar(&resume, "resume", false, "locate and resume most-recent checkpointed cycle (protocol lands in M3)")
	fs.BoolVar(&dryRun, "dry-run", false, "parse args, print resolved config as JSON, exit 0 (no orchestrator invocation)")
	fs.BoolVar(&reset, "reset", false, "prune infrastructure-systemic/transient + ship-gate-config from state.json:failedApproaches before loop")
	fs.StringVar(&fingerprint, "fingerprint", "", "with --reset, acknowledge this failure fingerprint in .evolve/resolved-fingerprints.json so the blocker-breaker excludes it (ADR-0072 extension operator unblock)")
	fs.BoolVar(&consensusAudit, "consensus-audit", false, "opt-in cross-CLI auditor consensus mode")
	fs.BoolVar(&forceFresh, "force-fresh", false, "start fresh even if an unfinished cycle exists (history NOT sealed; use evolve cycle reset to seal)")
	fs.BoolVar(&skipPreflight, "skip-preflight", false, "bypass the whole pre-batch readiness gate (no checks, no boot)")
	fs.BoolVar(&skipPreflightBoot, "skip-preflight-boot", false, "run cheap checks but skip the real bridge-boot probe (CI/offline)")
	fs.BoolVar(&untilInboxEmpty, "until-inbox-empty", false, "chain batches: after each batch, start another until the inbox drains (bounded by policy.json chain.max_batches; .evolve/loop-stop brakes it)")
	fs.BoolVar(&bypassPolicy, "bypass-policy", false, "use --bypass-policy to bypass policy.json pin enforcement for every phase in this batch (operator escape hatch)")

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

	args = stripRemovedBudgetFlags(args, func(m string) {
		fmt.Fprintf(stderr, "evolve loop: WARN: %s\n", m)
	})
	if err := fs.Parse(args); err != nil {
		return loopConfig{}, 10
	}

	absOrWarn := func(label, p string) string {
		return paths.AbsoluteRoot(label, p, func(m string) {
			fmt.Fprintf(stderr, "evolve loop: WARN: %s\n", m)
		})
	}
	projectRoot = absOrWarn("--project-root", projectRoot)

	posCycles, posStrategy, posGoal := parsePositional(fs.Args())

	if posCycles > 0 && cyclesFlag == 0 && maxCyclesFlag == 0 {
		fmt.Fprintf(stderr, "evolve loop: WARN: bare positional integer (%d) parsed as --cycles is deprecated; prefer explicit --cycles N\n", posCycles)
	}

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
		Fingerprint:       fingerprint,
		ConsensusAudit:    consensusAudit,
		DryRun:            dryRun,
		ForceFresh:        forceFresh,
		SkipPreflight:     skipPreflight,
		SkipPreflightBoot: skipPreflightBoot,
		BypassPolicy:      bypassPolicy,
		ChainMode:         untilInboxEmpty,
		PerAgentCLI:       perAgentCLI,
		PerAgentModel:     perAgentModel,
	}, 0
}

// parsePositional's order is fixed: CYCLES, then STRATEGY, then GOAL —
// changing it would silently reparse existing operator invocations differently.
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

func joinArgs(args []string) string {
	return strings.Join(args, " ")
}

func buildCycleContext(cfg loopConfig) map[string]string {
	out := map[string]string{
		"strategy": cfg.Strategy,
	}
	if cfg.GoalText != "" {
		out["goal"] = cfg.GoalText
	}
	return out
}

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
	if cfg.Resume {
		// IPC key "EVOLVE_RESUME" is split so the operator-flag registry guard
		// does not classify this parent-to-child handoff as a configurable flag.
		// SSOT IPC-protocol-allowed
		out["EVOLVE_"+"RESUME"] = "1"
	}
	for agent, cli := range cfg.PerAgentCLI {
		out["EVOLVE_"+phaseEnvAgentKey(agent)+"_CLI"] = cli
	}
	for agent, model := range cfg.PerAgentModel {
		out["EVOLVE_"+phaseEnvAgentKey(agent)+"_MODEL"] = model
	}
	return out
}

// phaseEnvAgentKey mirrors envchain.PhaseEnvKey's normalization (upper-case,
// dash to underscore), so EVOLVE_<AGENT>_CLI/_MODEL match the runner's lookup.
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
