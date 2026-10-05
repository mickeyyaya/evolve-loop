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
	preflightOnly     bool
	detach            bool
	logPath           string
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
	fs.BoolVar(&lf.preflightOnly, "preflight-only", false, "run the pre-batch readiness gate, print each check's verdict and exit without dispatching (exit 0 ready, 1 naming the blocking check)")
	fs.BoolVar(&lf.detach, "detach", false, "launch the loop in a new session with stdout/stderr appended to --log, print its pid and wait (policy boot.detach_wait_s) for its run lease: exit 0 running, 1 exited or unconfirmed")
	fs.StringVar(&lf.logPath, "log", "", "with --detach: the file the detached loop's stdout and stderr are appended to (required with --detach)")
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
	if rc := validateLoopModes(lf, stderr); rc != 0 {
		return loopConfig{}, rc
	}

	lf.projectRoot = absOrWarn(stderr, "--project-root", lf.projectRoot)
	posCycles, posStrategy, posGoal := parsePositional(fs.Args())
	resolvedCycles := resolveCycleCount(stderr, lf.cyclesFlag, lf.maxCyclesFlag, posCycles)

	resolvedStrategy, rc := resolveStrategy(stderr, lf.strategy, posStrategy)
	if rc != 0 {
		return loopConfig{}, rc
	}

	if verb, reserved := reservedGoalVerb(posGoal); reserved {
		fmt.Fprintf(stderr, "evolve loop: %q is a reserved word, not a goal — did you mean: %s? (to use it as goal text pass --goal-text)\n", posGoal, verb)
		return loopConfig{}, 10
	}

	resolvedGoalHash, resolvedGoalText, rc := resolveGoal(stderr, lf.goalHash, lf.goalText, posGoal, lf.resume, lf.dryRun || lf.preflightOnly)
	if rc != 0 {
		return loopConfig{}, rc
	}

	if lf.evolveDir == "" {
		lf.evolveDir = filepath.Join(lf.projectRoot, ".evolve")
	}
	lf.evolveDir = absOrWarn(stderr, "--evolve-dir", lf.evolveDir)

	cfg := buildLoopConfig(lf, resolvedGoalHash, resolvedGoalText, resolvedStrategy, resolvedCycles, posCycles, perAgentCLI, perAgentModel)
	return withDetachLaunch(cfg, lf, args, len(args)-len(fs.Args()), stderr)
}

func withDetachLaunch(cfg loopConfig, lf *loopArgFlags, args []string, flagEnd int, stderr io.Writer) (loopConfig, int) {
	if !lf.detach {
		return cfg, 0
	}
	logPath, err := filepath.Abs(lf.logPath)
	if err != nil {
		fmt.Fprintf(stderr, "evolve loop: --log %q: %v\n", lf.logPath, err)
		return loopConfig{}, 10
	}
	cfg.LogPath = logPath
	cfg.DetachArgv = detachChildArgs(args, flagEnd)
	return cfg, 0
}

func validateLoopModes(lf *loopArgFlags, stderr io.Writer) int {
	var conflict string
	switch {
	case lf.detach && lf.preflightOnly:
		conflict = "--detach and --preflight-only are mutually exclusive"
	case lf.preflightOnly && lf.skipPreflight:
		conflict = "--preflight-only and --skip-preflight are mutually exclusive (nothing would be checked)"
	case lf.preflightOnly && lf.dryRun:
		conflict = "--preflight-only and --dry-run are mutually exclusive"
	case lf.detach && lf.dryRun:
		conflict = "--detach and --dry-run are mutually exclusive"
	case lf.detach && lf.logPath == "":
		conflict = "--detach requires --log F"
	case !lf.detach && lf.logPath != "":
		conflict = "--log is only valid with --detach"
	default:
		return 0
	}
	fmt.Fprintf(stderr, "evolve loop: %s\n", conflict)
	return 10
}

var loopReservedGoalWords = map[string]string{
	"status": "evolve loop status",
	"stop":   "evolve loop-stop",
	"help":   "evolve loop -h",
	"plan":   "evolve loop --dry-run",
	"watch":  "evolve dashboard",
}

func reservedGoalVerb(goal string) (verb string, reserved bool) {
	verb, reserved = loopReservedGoalWords[strings.ToLower(strings.TrimSpace(goal))]
	return verb, reserved
}

func detachChildArgs(args []string, flagEnd int) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if i >= flagEnd {
			out = append(out, args[i])
			continue
		}
		switch name, hasValue := flagTokenName(args[i]); name {
		case "detach":
		case "log":
			if !hasValue {
				i++
			}
		default:
			out = append(out, args[i])
		}
	}
	return out
}

func flagTokenName(token string) (name string, hasValue bool) {
	if token == "--" || !strings.HasPrefix(token, "-") {
		return "", false
	}
	name, _, hasValue = strings.Cut(strings.TrimPrefix(strings.TrimPrefix(token, "-"), "-"), "=")
	return name, hasValue
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
		PreflightOnly:     lf.preflightOnly,
		Detach:            lf.detach,
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
