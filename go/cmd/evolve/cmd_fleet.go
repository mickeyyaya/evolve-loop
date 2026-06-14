// cmd_fleet.go — `evolve fleet` concurrent-cycle supervisor (ADR-0049 S6 / CE.2).
// Launches N cycles at the same time, each `evolve cycle run` in its OWN process
// with EVOLVE_FLEET=1, so the orchestrator skips the whole-cycle global lock
// (root-cause R1) and the per-resource flocks (S2–S5) serialize the shared
// writes. The missing producer for the EVOLVE_FLEET flag.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func runFleet(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve fleet", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		count       int
		goalHash    string
		concurrency int
		simulate    bool
	)
	fs.IntVar(&count, "count", 0, "number of concurrent cycles to launch (required)")
	fs.StringVar(&goalHash, "goal-hash", "", "goal hash passed to each cycle (required)")
	fs.IntVar(&concurrency, "concurrency", 0, "max concurrent cycles (0 = count)")
	fs.BoolVar(&simulate, "simulate", false, "no-LLM walk: each cycle returns PASS without calling out — validates the fleet concurrency plumbing (lock-skip, distinct cycle numbers, isolated worktrees, serialized ship) deterministically")
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
		Launch:      execCycleLaunch(binPath, simulate, stdout, stderr),
	}
	if err := sup.Validate(); err != nil {
		fmt.Fprintf(stderr, "evolve fleet: %v\n", err)
		return 1
	}
	specs := make([]fleet.CycleSpec, count)
	for i := range specs {
		specs[i] = fleet.CycleSpec{GoalHash: goalHash}
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
	fmt.Fprintf(stderr, "[fleet] %d/%d cycles ok\n", count-failed, count)
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
func cycleRunArgs(goalHash string, simulate bool) []string {
	args := []string{"cycle", "run", "--goal-hash", goalHash}
	if simulate {
		args = append(args, "-simulate")
	}
	return args
}

func execCycleLaunch(binPath string, simulate bool, stdout, stderr io.Writer) fleet.LaunchFn {
	return func(ctx context.Context, spec fleet.CycleSpec) (int, error) {
		cmd := exec.CommandContext(ctx, binPath, cycleRunArgs(spec.GoalHash, simulate)...)
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
