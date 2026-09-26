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
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

// loadPlanSpecs parses an `evolve fleet --plan` backlog into up to `count`
// disjoint-scoped cycle specs stamped with goalHash; the second return is the
// deferred backlog for a later wave.
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

func runFleet(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "soak" {
		return runFleetSoak(args[1:], stdin, stdout, stderr)
	}
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
		Launch:      execCycleLaunch(binPath, simulate, "", goalHash, "", stdout, stderr),
	}
	if err := sup.Validate(); err != nil {
		fmt.Fprintf(stderr, "evolve fleet: %v\n", err)
		return 1
	}

	var specs []fleet.CycleSpec
	if planPath != "" {
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

// cycleRunArgs builds the `evolve cycle run` argv for one fleet cycle. Pure +
// testable; --simulate threads the no-LLM -simulate flag through.
func cycleRunArgs(goalHash, outputContract, goalText string, simulate bool, projectRoot string) []string {
	args := []string{"cycle", "run", "--goal-hash", goalHash}
	if outputContract != "" {
		args = append(args, "--goal", outputContract)
	} else if goalText != "" {
		args = append(args, "--goal", goalText)
	}
	if simulate {
		args = append(args, "-simulate")
	}
	if projectRoot != "" {
		args = append(args, "--project-root", projectRoot)
	}
	return args
}

// laneGoalHash prefers the spec's own GoalHash over the wave/campaign-level
// fallback the launcher was built with.
func laneGoalHash(specGoalHash, fallback string) string {
	if specGoalHash != "" {
		return specGoalHash
	}
	return fallback
}

func execCycleLaunch(binPath string, simulate bool, projectRoot, goalHash, goalText string, stdout, stderr io.Writer) fleet.LaunchFn {
	var logMu sync.Mutex // serializes interleaved output across concurrent cycles
	return func(ctx context.Context, spec fleet.CycleSpec) (int, error) {
		prefix := "[" + cycleLogTag(spec) + "] "
		ow := &prefixLineWriter{w: stdout, prefix: prefix, mu: &logMu}
		ew := &prefixLineWriter{w: stderr, prefix: prefix, mu: &logMu}
		defer func() { ow.Flush(); ew.Flush() }()
		cmd := exec.CommandContext(ctx, binPath, cycleRunArgs(laneGoalHash(spec.GoalHash, goalHash), spec.OutputContract, goalText, simulate, projectRoot)...)
		cmd.Env = append(os.Environ(), envPairs(spec.Env)...)
		cmd.Stdout = ow
		cmd.Stderr = ew
		// On ctx cancel/timeout, SIGTERM the child for a graceful exit; WaitDelay
		// escalates to SIGKILL if it ignores the signal, so a wedged cycle is reaped.
		cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
		cmd.WaitDelay = 10 * time.Second
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

func cycleLogTag(spec fleet.CycleSpec) string {
	if len(spec.Scope) > 0 {
		return strings.Join(spec.Scope, "+")
	}
	if len(spec.GoalHash) >= 8 {
		return spec.GoalHash[:8]
	}
	return spec.GoalHash
}

func envPairs(overlay map[string]string) []string {
	out := make([]string, 0, len(overlay))
	for k, v := range overlay {
		out = append(out, k+"="+v)
	}
	return out
}
