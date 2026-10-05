package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/phasecmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

var buildHandoffFloorFor phasecmd.BuildHandoffFloorFor = probeBuildHandoffFloor

func runPhase(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return phasecmd.NewRunPhase(buildHandoffFloorFor)(args, stdin, stdout, stderr)
}

func runSelfcheck(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 || args[0] != "build" {
		fmt.Fprintln(stderr, "usage: evolve selfcheck build [--worktree DIR] — the build self-check (the deliverable contract plus the build handoff floor, bound to the worktree's cycle; the same probe as `evolve phase verify build`); iterate until GREEN before declaring done (ADR-0076, ADR-0117)")
		return 2
	}
	fs := flag.NewFlagSet("selfcheck build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	worktree := fs.String("worktree", "", "worktree to check (default: current directory)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	dir := *worktree
	if dir == "" {
		if cwd, err := os.Getwd(); err == nil {
			dir = cwd
		}
	}
	probe := core.BuildHandoffProbe{EvolveDir: filepath.Join(dir, ".evolve"), ProjectRoot: cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT"), Worktree: dir}
	checked, err := phasecmd.BuildSelfCheck{Floor: buildHandoffFloorFor, Probe: probe}.Verify(stderr)
	if err != nil {
		fmt.Fprintf(stderr, "[selfcheck] the build handoff cannot be judged: %v\n", err)
		return 2
	}
	if !checked.OK {
		fmt.Fprintf(stdout, "[selfcheck] %d finding(s) — fix these exactly, then re-run until GREEN:\n", len(checked.Violations))
		for _, v := range checked.Violations {
			fmt.Fprintf(stdout, "  [%s] %s\n", v.Code, v.Message)
		}
		return 1
	}
	fmt.Fprintln(stdout, selfcheckGreen(dir, checked.Unbound))
	return 0
}

func selfcheckGreen(dir string, unbound error) string {
	if unbound != nil {
		return fmt.Sprintf("[selfcheck] GREEN without a cycle binding for %s — not a prediction of the build handoff floor", dir)
	}
	return fmt.Sprintf("[selfcheck] GREEN: build handoff floor checks pass for %s — safe to hand off", dir)
}
