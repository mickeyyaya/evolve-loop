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

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var fleetLaunchFactory = execCycleLaunch

var fleetExecutable = os.Executable

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
	fs.StringVar(&planPath, "plan", "", "advisor backlog JSON ([{\"id\",\"files\"}]); partitioned into <=count disjoint-scoped cycles so each works independent files (ADR-0049 E). Required without --simulate: each lane needs the lane-scope.json pin its plan writes. A --simulate run without it launches count identical cycles.")
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
	if planPath == "" && !simulate {
		fmt.Fprintln(stderr, "evolve fleet: --plan is required without --simulate: a lane with no plan has no lane-scope.json pin, so its triage refuses with TRIAGE_LANE_PIN_MISSING")
		return 1
	}
	binPath, err := fleetExecutable()
	if err != nil {
		fmt.Fprintf(stderr, "evolve fleet: cannot resolve binary: %v\n", err)
		return 1
	}

	sup := &fleet.Supervisor{
		Concurrency: concurrency,
		Launch:      fleetLaunchFactory(binPath, simulate, "", goalHash, "", stdout, stderr),
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
	return reportFleetResults(runLanesThenPublish(context.Background(), sup, specs, "", stderr), stderr)
}

func reportFleetResults(results []fleet.Result, stderr io.Writer) int {
	for _, r := range results {
		fmt.Fprintf(stderr, "[fleet] cycle %d: %s (exit=%d, err=%v)\n", r.Index, r.Status(), r.ExitCode, r.Err)
	}
	t := laneTally(results)
	fmt.Fprintf(stderr, "[fleet] %d/%d cycles ok%s\n", t.OK, t.Total(), t.DeferredSuffix())
	if t.Failed > 0 {
		return 1
	}
	return 0
}

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

func laneGoalHash(specGoalHash, fallback string) string {
	if specGoalHash != "" {
		return specGoalHash
	}
	return fallback
}

func execCycleLaunch(binPath string, simulate bool, projectRoot, goalHash, goalText string, stdout, stderr io.Writer) fleet.LaunchFn {
	var logMu sync.Mutex
	return func(ctx context.Context, spec fleet.CycleSpec) (int, error) {
		prefix := "[" + cycleLogTag(spec) + "] "
		ow := &prefixLineWriter{w: stdout, prefix: prefix, mu: &logMu}
		ew := &prefixLineWriter{w: stderr, prefix: prefix, mu: &logMu}
		defer func() { ow.Flush(); ew.Flush() }()
		cmd := sysexec.Command(ctx, binPath, cycleRunArgs(laneGoalHash(spec.GoalHash, goalHash), spec.OutputContract, goalText, simulate, projectRoot)...)
		cmd.Env = append(os.Environ(), envPairs(spec.Env)...)
		cmd.Stdout = ow
		cmd.Stderr = ew
		cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
		cmd.WaitDelay = core.LaneLaunchWaitDelay
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
