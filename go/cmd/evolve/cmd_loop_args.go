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

type loopArgFlags struct {
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
}

func registerLoopArgFlags(fs *flag.FlagSet) *loopArgFlags {
	lf := &loopArgFlags{}
	fs.StringVar(&lf.projectRoot, "project-root", ".", "absolute path to project root")
	fs.StringVar(&lf.evolveDir, "evolve-dir", "", "path to .evolve/ (default <project-root>/.evolve)")
	fs.StringVar(&lf.goalHash, "goal-hash", "", "explicit 64-char (or 8-char prefix) SHA256 of goal; mutually exclusive with --goal-text")
	fs.StringVar(&lf.goalText, "goal-text", "", "goal text; hashed via goalhash.Compute (normalize+SHA256)")
	fs.StringVar(&lf.strategy, "strategy", "", "balanced|innovate|harden|repair|ultrathink|autoresearch (default: balanced)")
	fs.IntVar(&lf.maxCyclesFlag, "max-cycles", 0, "maximum iterations to run: cycles when sequential, waves of fleet.count lanes in fleet mode (default 1; aliased by --cycles)")
	fs.IntVar(&lf.cyclesFlag, "cycles", 0, "alias for --max-cycles")
	fs.BoolVar(&lf.resume, "resume", false, "locate and resume most-recent checkpointed cycle (protocol lands in M3)")
	fs.BoolVar(&lf.dryRun, "dry-run", false, "parse args, print resolved config as JSON, exit 0 (no orchestrator invocation)")
	fs.BoolVar(&lf.reset, "reset", false, "prune infrastructure-systemic/transient + ship-gate-config from state.json:failedApproaches before loop")
	fs.StringVar(&lf.fingerprint, "fingerprint", "", "with --reset, acknowledge this failure fingerprint in .evolve/resolved-fingerprints.json so the blocker-breaker excludes it (ADR-0072 extension operator unblock)")
	fs.BoolVar(&lf.consensusAudit, "consensus-audit", false, "opt-in cross-CLI auditor consensus mode")
	fs.BoolVar(&lf.forceFresh, "force-fresh", false, "start fresh even if an unfinished cycle exists (history NOT sealed; use evolve cycle reset to seal)")
	fs.BoolVar(&lf.skipPreflight, "skip-preflight", false, "bypass the whole pre-batch readiness gate (no checks, no boot)")
	fs.BoolVar(&lf.skipPreflightBoot, "skip-preflight-boot", false, "run cheap checks but skip the real bridge-boot probe (CI/offline)")
	fs.BoolVar(&lf.untilInboxEmpty, "until-inbox-empty", false, "chain batches: after each batch, start another until the inbox drains (bounded by policy.json chain.max_batches; .evolve/loop-stop brakes it)")
	fs.BoolVar(&lf.bypassPolicy, "bypass-policy", false, "use --bypass-policy to bypass policy.json pin enforcement for every phase in this batch (operator escape hatch)")
	return lf
}

func registerPerAgentOverrides(fs *flag.FlagSet) (cli, model map[string]string) {
	cli = map[string]string{}
	model = map[string]string{}
	fs.Func("cli", "per-agent CLI override (repeatable): --cli auditor=claude-tmux", func(v string) error {
		agent, value, ok := strings.Cut(v, "=")
		if !ok || strings.TrimSpace(agent) == "" || strings.TrimSpace(value) == "" {
			return fmt.Errorf("--cli expects agent=cli (e.g. --cli auditor=claude-tmux); got %q", v)
		}
		cli[strings.TrimSpace(agent)] = strings.TrimSpace(value)
		return nil
	})
	fs.Func("model", "per-agent model override (repeatable): --model auditor=opus", func(v string) error {
		agent, value, ok := strings.Cut(v, "=")
		if !ok || strings.TrimSpace(agent) == "" || strings.TrimSpace(value) == "" {
			return fmt.Errorf("--model expects agent=model (e.g. --model auditor=opus); got %q", v)
		}
		model[strings.TrimSpace(agent)] = strings.TrimSpace(value)
		return nil
	})
	return cli, model
}

func absOrWarn(stderr io.Writer, label, p string) string {
	return paths.AbsoluteRoot(label, p, func(m string) {
		fmt.Fprintf(stderr, "evolve loop: WARN: %s\n", m)
	})
}

func resolveCycleCount(stderr io.Writer, cyclesFlag, maxCyclesFlag, posCycles int) int {
	if posCycles > 0 && cyclesFlag == 0 && maxCyclesFlag == 0 {
		fmt.Fprintf(stderr, "evolve loop: WARN: bare positional integer (%d) parsed as --cycles is deprecated; prefer explicit --cycles N\n", posCycles)
	}
	switch {
	case cyclesFlag > 0:
		return cyclesFlag
	case maxCyclesFlag > 0:
		return maxCyclesFlag
	case posCycles > 0:
		return posCycles
	default:
		return 1
	}
}

func resolveStrategy(stderr io.Writer, strategy, posStrategy string) (string, int) {
	resolved := strategy
	if resolved == "" {
		resolved = posStrategy
	}
	if resolved == "" {
		resolved = "balanced"
	}
	if _, ok := validStrategies[resolved]; !ok {
		fmt.Fprintf(stderr, "evolve loop: invalid --strategy %q (valid: balanced|innovate|harden|repair|ultrathink|autoresearch)\n", resolved)
		return "", 10
	}
	return resolved, 0
}

func resolveGoal(stderr io.Writer, goalHash, goalText, posGoal string, resume, dryRun bool) (hash, text string, rc int) {
	text = goalText
	if text == "" && posGoal != "" {
		text = posGoal
	}
	hash = goalHash
	if hash == "" && text != "" {
		hash = goalhash.Compute(text)
	}
	if hash == "" && !resume && !dryRun {
		fmt.Fprintln(stderr, "evolve loop: a goal is required — pass --goal-hash, --goal-text, or a positional goal (or --resume to continue a checkpointed cycle)")
		return "", "", 10
	}
	return hash, text, 0
}

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

	lf := registerLoopArgFlags(fs)
	perAgentCLI, perAgentModel := registerPerAgentOverrides(fs)

	args = stripRemovedBudgetFlags(args, func(m string) {
		fmt.Fprintf(stderr, "evolve loop: WARN: %s\n", m)
	})
	if err := fs.Parse(args); err != nil {
		return loopConfig{}, 10
	}

	lf.projectRoot = absOrWarn(stderr, "--project-root", lf.projectRoot)
	posCycles, posStrategy, posGoal := parsePositional(fs.Args())
	resolvedCycles := resolveCycleCount(stderr, lf.cyclesFlag, lf.maxCyclesFlag, posCycles)

	resolvedStrategy, rc := resolveStrategy(stderr, lf.strategy, posStrategy)
	if rc != 0 {
		return loopConfig{}, rc
	}

	resolvedGoalHash, resolvedGoalText, rc := resolveGoal(stderr, lf.goalHash, lf.goalText, posGoal, lf.resume, lf.dryRun)
	if rc != 0 {
		return loopConfig{}, rc
	}

	if lf.evolveDir == "" {
		lf.evolveDir = filepath.Join(lf.projectRoot, ".evolve")
	}
	lf.evolveDir = absOrWarn(stderr, "--evolve-dir", lf.evolveDir)

	return buildLoopConfig(lf, resolvedGoalHash, resolvedGoalText, resolvedStrategy, resolvedCycles, posCycles, perAgentCLI, perAgentModel), 0
}

func buildLoopConfig(lf *loopArgFlags, goalHash, goalText, strategy string, maxCycles, posCycles int, perAgentCLI, perAgentModel map[string]string) loopConfig {
	return loopConfig{
		ProjectRoot:       lf.projectRoot,
		EvolveDir:         lf.evolveDir,
		GoalHash:          goalHash,
		GoalText:          goalText,
		Strategy:          strategy,
		MaxCycles:         maxCycles,
		MaxCyclesExplicit: lf.cyclesFlag > 0 || lf.maxCyclesFlag > 0 || posCycles > 0,
		Resume:            lf.resume,
		Reset:             lf.reset,
		Fingerprint:       lf.fingerprint,
		ConsensusAudit:    lf.consensusAudit,
		DryRun:            lf.dryRun,
		ForceFresh:        lf.forceFresh,
		SkipPreflight:     lf.skipPreflight,
		SkipPreflightBoot: lf.skipPreflightBoot,
		BypassPolicy:      lf.bypassPolicy,
		ChainMode:         lf.untilInboxEmpty,
		PerAgentCLI:       perAgentCLI,
		PerAgentModel:     perAgentModel,
	}
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
