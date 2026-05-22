// `evolve loop` drives the cycle dispatcher loop with batch budget
// enforcement. Sequential by design — each cycle blocks the next until
// it completes or trips the batch cap (matches v8.34.0+ bash
// dispatcher behavior).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// runLoop implements `evolve loop`.
func runLoop(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve loop", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		projectRoot string
		evolveDir   string
		goalHash    string
		maxCycles   int
		budgetUSD   float64
		batchCapUSD float64
	)
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to project root")
	fs.StringVar(&evolveDir, "evolve-dir", "", "path to .evolve/ (default <project-root>/.evolve)")
	fs.StringVar(&goalHash, "goal-hash", "", "8-char SHA256 of the goal (required)")
	fs.IntVar(&maxCycles, "max-cycles", 1, "maximum cycles to run")
	fs.Float64Var(&budgetUSD, "budget-usd", 999999, "per-cycle USD budget cap")
	fs.Float64Var(&batchCapUSD, "batch-cap-usd", 20.0, "cumulative batch USD cap (trips with non-zero exit)")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if goalHash == "" {
		fmt.Fprintln(stderr, "evolve loop: --goal-hash is required")
		return 10
	}
	if evolveDir == "" {
		evolveDir = filepath.Join(projectRoot, ".evolve")
	}
	if maxCycles < 1 {
		maxCycles = 1
	}

	orch := wireOrchestrator(projectRoot, evolveDir)

	type loopResult struct {
		StopReason string             `json:"stop_reason"`
		Cycles     []core.CycleResult `json:"cycles"`
		TotalCost  float64            `json:"total_cost_usd"`
	}
	lr := loopResult{StopReason: "max_cycles"}

	for i := 0; i < maxCycles; i++ {
		req := core.CycleRequest{
			ProjectRoot: projectRoot,
			GoalHash:    goalHash,
			Budget: core.BudgetEnvelope{
				MaxUSD:      budgetUSD,
				BatchCapUSD: batchCapUSD,
			},
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
