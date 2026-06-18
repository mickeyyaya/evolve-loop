// cmd_fleet.go — `evolve fleet` concurrent-cycle supervisor (ADR-0049 S6 / CE.2).
// Launches N cycles at the same time, each `evolve cycle run` in its OWN process
// with EVOLVE_FLEET=1, so the orchestrator skips the whole-cycle global lock
// (root-cause R1) and the per-resource flocks (S2–S5) serialize the shared
// writes. The missing producer for the EVOLVE_FLEET flag.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

// loadPlanSpecs parses an `evolve fleet --plan` backlog ([{"id","files"}]) and
// partitions it into at most `count` disjoint-scoped cycle specs (ADR-0049 E:
// the advisor assigns independent todos to independent cycles), each stamped
// with goalHash. Returns the launch specs and the deferred todos (run in a later
// wave). The partition guarantees every file is owned by one cycle, so the
// launched cycles never collide on the shared tree.
func loadPlanSpecs(planJSON []byte, goalHash string, count int) ([]fleet.CycleSpec, []fleet.Todo, error) {
	var todos []fleet.Todo
	if err := json.Unmarshal(planJSON, &todos); err != nil {
		return nil, nil, fmt.Errorf("parse --plan backlog: %w", err)
	}
	specs, deferred := fleet.PlanCycles(todos, count)
	for i := range specs {
		specs[i].GoalHash = goalHash
	}
	return specs, deferred, nil
}

func runFleet(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve fleet", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		count       int
		goalHash    string
		concurrency int
		simulate    bool
		planPath    string
	)
	fs.IntVar(&count, "count", 0, "number of concurrent cycles to launch (required)")
	fs.StringVar(&goalHash, "goal-hash", "", "goal hash passed to each cycle (required)")
	fs.IntVar(&concurrency, "concurrency", 0, "max concurrent cycles (0 = count)")
	fs.BoolVar(&simulate, "simulate", false, "no-LLM walk: each cycle returns PASS without calling out — validates the fleet concurrency plumbing (lock-skip, distinct cycle numbers, isolated worktrees, serialized ship) deterministically")
	fs.StringVar(&planPath, "plan", "", "advisor backlog JSON ([{\"id\",\"files\"}]); partitioned into <=count disjoint-scoped cycles so each works independent files (ADR-0049 E). Without it, count identical cycles launch.")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if count <= 0 {
		fmt.Fprintln(stderr, "evolve fleet: --count must be > 0")
		return 1
	}
	if goalHash == "" {
		fmt.Fprintln(stderr, "evolve fleet: --goal-hash is required")
		return 1
	}
	binPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "evolve fleet: cannot resolve binary: %v\n", err)
		return 1
	}

	sup := &fleet.Supervisor{
		Concurrency: concurrency,
		Launch:      execCycleLaunch(binPath, simulate, "", stdout, stderr),
	}
	if err := sup.Validate(); err != nil {
		fmt.Fprintf(stderr, "evolve fleet: %v\n", err)
		return 1
	}

	var specs []fleet.CycleSpec
	if planPath != "" {
		// Advisor-partitioned: each cycle gets a DISJOINT todo subset so the
		// concurrent cycles never edit the same file (ADR-0049 E).
		planJSON, rerr := os.ReadFile(planPath)
		if rerr != nil {
			fmt.Fprintf(stderr, "evolve fleet: read --plan %s: %v\n", planPath, rerr)
			return 1
		}
		planned, deferred, perr := loadPlanSpecs(planJSON, goalHash, count)
		if perr != nil {
			fmt.Fprintf(stderr, "evolve fleet: %v\n", perr)
			return 1
		}
		if len(planned) == 0 {
			fmt.Fprintln(stderr, "evolve fleet: --plan yielded no schedulable cycles (empty backlog?)")
			return 1
		}
		specs = planned
		for _, td := range deferred {
			fmt.Fprintf(stderr, "[fleet] deferred to a later wave (bridges concurrent cycles): %s\n", td.ID)
		}
		fmt.Fprintf(stderr, "[fleet] advisor plan: %d disjoint cycles, %d deferred\n", len(specs), len(deferred))
	} else {
		// No plan: count identical cycles. The per-resource locks keep them SAFE,
		// but they may pick overlapping work (S5b rebase-reaudit is the net).
		specs = make([]fleet.CycleSpec, count)
		for i := range specs {
			specs[i] = fleet.CycleSpec{GoalHash: goalHash}
		}
	}
	results := sup.Run(context.Background(), specs)

	failed := 0
	for _, r := range results {
		status := "ok"
		if r.Err != nil || r.ExitCode != 0 {
			status, failed = "FAIL", failed+1
		}
		fmt.Fprintf(stderr, "[fleet] cycle %d: %s (exit=%d, err=%v)\n", r.Index, status, r.ExitCode, r.Err)
	}
	fmt.Fprintf(stderr, "[fleet] %d/%d cycles ok\n", len(specs)-failed, len(specs))
	if failed > 0 {
		return 1
	}
	return 0
}

// execCycleLaunch returns a fleet.LaunchFn that runs one `evolve cycle run` in a
// child process. The child inherits the parent env plus the supervisor's
// per-spec overlay (which already forced EVOLVE_FLEET=1).
// cycleRunArgs builds the `evolve cycle run` argv for one fleet cycle. Pure +
// testable; --simulate threads the no-LLM -simulate flag through.
func cycleRunArgs(goalHash string, simulate bool, projectRoot string) []string {
	args := []string{"cycle", "run", "--goal-hash", goalHash}
	if simulate {
		args = append(args, "-simulate")
	}
	if projectRoot != "" {
		args = append(args, "--project-root", projectRoot)
	}
	return args
}

func execCycleLaunch(binPath string, simulate bool, projectRoot string, stdout, stderr io.Writer) fleet.LaunchFn {
	return func(ctx context.Context, spec fleet.CycleSpec) (int, error) {
		cmd := exec.CommandContext(ctx, binPath, cycleRunArgs(spec.GoalHash, simulate, projectRoot)...)
		cmd.Env = append(os.Environ(), envPairs(spec.Env)...)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				return ee.ExitCode(), nil
			}
			return -1, err
		}
		return 0, nil
	}
}

func envPairs(overlay map[string]string) []string {
	out := make([]string, 0, len(overlay))
	for k, v := range overlay {
		out = append(out, k+"="+v)
	}
	return out
}
