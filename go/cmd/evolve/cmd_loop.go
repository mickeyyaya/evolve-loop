// `evolve loop` drives the cycle dispatcher loop with batch budget
// enforcement. Sequential by design — each cycle blocks the next until
// it completes or trips the batch cap (matches v8.34.0+ bash
// dispatcher behavior).
//
// v11.5.0 M1: CLI surface extended to mirror legacy/scripts/dispatch/
// evolve-loop-dispatch.sh — positional args ([CYCLES] [STRATEGY]
// [GOAL...]), --goal-text (computes hash via goalhash.Compute),
// --strategy, --resume (flag plumbing only; protocol lands in M3),
// --dry-run, --reset, --consensus-audit. Existing --goal-hash callers
// continue to work unchanged.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/goalhash"
)

// validStrategies mirrors the bash whitelist at
// legacy/scripts/dispatch/evolve-loop-dispatch.sh:294-298.
var validStrategies = map[string]struct{}{
	"balanced":     {},
	"innovate":     {},
	"harden":       {},
	"repair":       {},
	"ultrathink":   {},
	"autoresearch": {},
}

// loopConfig is the resolved invocation. Extracted so --dry-run and
// tests can inspect what would be done without invoking the
// orchestrator.
type loopConfig struct {
	ProjectRoot     string  `json:"project_root"`
	EvolveDir       string  `json:"evolve_dir"`
	GoalHash        string  `json:"goal_hash"`
	GoalText        string  `json:"goal_text,omitempty"`
	Strategy        string  `json:"strategy"`
	MaxCycles       int     `json:"max_cycles"`
	BudgetUSD       float64 `json:"budget_usd"`
	BatchCapUSD     float64 `json:"batch_cap_usd"`
	Resume          bool    `json:"resume,omitempty"`
	Reset           bool    `json:"reset,omitempty"`
	ConsensusAudit  bool    `json:"consensus_audit,omitempty"`
	DryRun          bool    `json:"dry_run,omitempty"`
	BudgetDriven    bool    `json:"budget_driven,omitempty"`
}

// runLoop implements `evolve loop`.
func runLoop(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	cfg, rc := parseLoopArgs(args, stderr)
	if rc != 0 {
		return rc
	}

	if cfg.DryRun {
		buf, _ := json.MarshalIndent(map[string]any{
			"dry_run": true,
			"config":  cfg,
		}, "", "  ")
		fmt.Fprintln(stdout, string(buf))
		return 0
	}

	orch := wireOrchestrator(cfg.ProjectRoot, cfg.EvolveDir)

	// Strategy + bypass-env propagation. Subagents read EVOLVE_STRATEGY
	// to select their prompt variant; Scout also reads Context["strategy"].
	cycleEnv := map[string]string{
		"EVOLVE_STRATEGY": cfg.Strategy,
	}
	if cfg.ConsensusAudit {
		cycleEnv["EVOLVE_CONSENSUS_AUDIT"] = "1"
	}
	if cfg.Resume {
		cycleEnv["EVOLVE_RESUME"] = "1"
	}
	if cfg.Reset {
		cycleEnv["EVOLVE_RESET"] = "1"
	}
	cycleCtx := map[string]string{
		"strategy": cfg.Strategy,
	}

	type loopResult struct {
		StopReason string             `json:"stop_reason"`
		Cycles     []core.CycleResult `json:"cycles"`
		TotalCost  float64            `json:"total_cost_usd"`
		Resumed    bool               `json:"resumed,omitempty"`
	}
	lr := loopResult{StopReason: "max_cycles"}

	// --resume short-circuits the loop: load the checkpoint, run one
	// cycle from the paused phase, then exit. M3 protocol.
	if cfg.Resume {
		lr.Resumed = true
		rp, err := core.LoadResumeState(context.Background(), cfg.ProjectRoot, cfg.EvolveDir, core.ResumeOptions{})
		if err != nil {
			fmt.Fprintf(stderr, "evolve loop: resume: %v\n", err)
			lr.StopReason = "error"
			buf, _ := json.MarshalIndent(lr, "", "  ")
			fmt.Fprintln(stdout, string(buf))
			return 2
		}
		fmt.Fprintf(stderr, "[resume] cycle=%d phase=%s reason=%s cost=$%.2f\n",
			rp.CycleID, rp.Phase, rp.Reason, rp.CostAtPause)
		req := core.CycleRequest{
			ProjectRoot: cfg.ProjectRoot,
			GoalHash:    cfg.GoalHash,
			Budget: core.BudgetEnvelope{
				MaxUSD:      cfg.BudgetUSD,
				BatchCapUSD: cfg.BatchCapUSD,
			},
			Env:     cycleEnv,
			Context: cycleCtx,
		}
		result, err := orch.RunCycleFromPhase(context.Background(), req, rp)
		lr.Cycles = append(lr.Cycles, result)
		if err != nil {
			lr.StopReason = "error"
			fmt.Fprintf(stderr, "evolve loop: resume cycle %d: %v\n", result.Cycle, err)
		} else if result.FinalVerdict == core.VerdictFAIL {
			lr.StopReason = "fail"
		} else {
			lr.StopReason = "resumed_complete"
		}
		buf, _ := json.MarshalIndent(lr, "", "  ")
		fmt.Fprintln(stdout, string(buf))
		if lr.StopReason == "error" || lr.StopReason == "fail" {
			return 2
		}
		return 0
	}

	for i := 0; i < cfg.MaxCycles; i++ {
		req := core.CycleRequest{
			ProjectRoot: cfg.ProjectRoot,
			GoalHash:    cfg.GoalHash,
			Budget: core.BudgetEnvelope{
				MaxUSD:      cfg.BudgetUSD,
				BatchCapUSD: cfg.BatchCapUSD,
			},
			Env:     cycleEnv,
			Context: cycleCtx,
		}
		result, err := orch.RunCycle(context.Background(), req)
		lr.Cycles = append(lr.Cycles, result)
		if err != nil {
			lr.StopReason = "error"
			fmt.Fprintf(stderr, "evolve loop: cycle %d: %v\n", result.Cycle, err)
			break
		}
		if result.FinalVerdict == core.VerdictFAIL {
			lr.StopReason = "fail"
			break
		}
	}

	buf, _ := json.MarshalIndent(lr, "", "  ")
	fmt.Fprintln(stdout, string(buf))
	if lr.StopReason == "error" || lr.StopReason == "fail" {
		return 2
	}
	return 0
}

// parseLoopArgs parses `evolve loop` arguments per the v11.5.0 M1 CLI
// surface. Returns the resolved config + rc (0 = success, 10 = bad
// args, exits printed to stderr).
//
// Argument precedence:
//
//	--goal-hash takes priority over --goal-text (--goal-text computes hash)
//	--goal-text takes priority over positional [GOAL...]
//	--cycles / --max-cycles take priority over positional [CYCLES]
//	--strategy takes priority over positional [STRATEGY]
//
// Positional parsing matches the bash dispatcher heuristic at
// legacy/scripts/dispatch/evolve-loop-dispatch.sh:325-349:
//
//	first numeric token (if any) → CYCLES
//	next token if matching strategy whitelist → STRATEGY
//	remaining tokens (joined by space) → GOAL
func parseLoopArgs(args []string, stderr io.Writer) (loopConfig, int) {
	fs := flag.NewFlagSet("evolve loop", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		projectRoot     string
		evolveDir       string
		goalHash        string
		goalText        string
		strategy        string
		maxCyclesFlag   int
		cyclesFlag      int
		budgetUSD       float64
		batchCapUSD     float64
		resume          bool
		dryRun          bool
		reset           bool
		consensusAudit  bool
	)
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to project root")
	fs.StringVar(&evolveDir, "evolve-dir", "", "path to .evolve/ (default <project-root>/.evolve)")
	fs.StringVar(&goalHash, "goal-hash", "", "explicit 64-char (or 8-char prefix) SHA256 of goal; mutually exclusive with --goal-text")
	fs.StringVar(&goalText, "goal-text", "", "goal text; hashed via goalhash.Compute (normalize+SHA256)")
	fs.StringVar(&strategy, "strategy", "", "balanced|innovate|harden|repair|ultrathink|autoresearch (default: balanced)")
	fs.IntVar(&maxCyclesFlag, "max-cycles", 0, "maximum cycles to run (default 1; aliased by --cycles)")
	fs.IntVar(&cyclesFlag, "cycles", 0, "alias for --max-cycles")
	fs.Float64Var(&budgetUSD, "budget-usd", 0, "per-cycle USD budget cap (default 999999)")
	fs.Float64Var(&batchCapUSD, "batch-cap-usd", 20.0, "cumulative batch USD cap (trips with non-zero exit)")
	fs.BoolVar(&resume, "resume", false, "locate and resume most-recent checkpointed cycle (protocol lands in M3)")
	fs.BoolVar(&dryRun, "dry-run", false, "parse args, print resolved config as JSON, exit 0 (no orchestrator invocation)")
	fs.BoolVar(&reset, "reset", false, "prune infrastructure-systemic/transient + ship-gate-config from state.json:failedApproaches before loop")
	fs.BoolVar(&consensusAudit, "consensus-audit", false, "opt-in cross-CLI auditor consensus mode")

	if err := fs.Parse(args); err != nil {
		return loopConfig{}, 10
	}

	// Parse positional args: [CYCLES] [STRATEGY] [GOAL...]
	posCycles, posStrategy, posGoal := parsePositional(fs.Args())

	// Resolve cycles: explicit flag > positional > default
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
	// Resume mode is the one path that doesn't require an explicit goal —
	// the resume protocol reads goal from cycle-state.json.
	if resolvedGoalHash == "" && !resume {
		fmt.Fprintln(stderr, "evolve loop: a goal is required — pass --goal-hash, --goal-text, or a positional goal (or --resume to continue a checkpointed cycle)")
		return loopConfig{}, 10
	}

	// Resolve budget: default 999999 (effectively no per-cycle cap).
	resolvedBudget := budgetUSD
	if resolvedBudget == 0 {
		resolvedBudget = 999999
	}
	budgetDriven := budgetUSD > 0 && budgetUSD < 999999

	// Resolve evolve-dir.
	if evolveDir == "" {
		evolveDir = filepath.Join(projectRoot, ".evolve")
	}

	return loopConfig{
		ProjectRoot:    projectRoot,
		EvolveDir:      evolveDir,
		GoalHash:       resolvedGoalHash,
		GoalText:       resolvedGoalText,
		Strategy:       resolvedStrategy,
		MaxCycles:      resolvedCycles,
		BudgetUSD:      resolvedBudget,
		BatchCapUSD:    batchCapUSD,
		Resume:         resume,
		Reset:          reset,
		ConsensusAudit: consensusAudit,
		DryRun:         dryRun,
		BudgetDriven:   budgetDriven,
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
// who paste their `/evolve-loop 3 balanced "fix bug"` invocations into
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

// joinArgs joins args with a single space, preserving inner quoting
// the way bash would when the operator quoted the goal in the original
// CLI invocation. Empty slice → empty string.
func joinArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	if len(args) == 1 {
		return args[0]
	}
	out := args[0]
	for _, a := range args[1:] {
		out += " " + a
	}
	return out
}
