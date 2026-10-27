package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

type cycleResumeResult struct {
	Cycle  int    `json:"cycle"`
	Phase  string `json:"phase"`
	Reason string `json:"reason"`
	State  string `json:"state"`
	DryRun bool   `json:"dry_run"`
}

var cycleResumeLoad = func(root string, log io.Writer) (*core.ResumePoint, error) {
	return core.LoadResumeState(context.Background(), root, paths.EvolveDirOf(root), core.ResumeOptions{Log: log})
}

func runCycleResume(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve cycle resume", flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectRoot := fs.String("project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	cycle := fs.Int("cycle", 0, "resume only if the paused cycle is this one")
	dryRun := fs.Bool("dry-run", false, "name the paused cycle and start nothing")
	asJSON := fs.Bool("json", false, "print the result as JSON")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 || *cycle < 0 {
		fmt.Fprintln(stderr, "usage: evolve cycle resume [--cycle N] [--dry-run] [--json] [--project-root P]")
		return cliExitUsage
	}
	root, err := loopStopRoot(*projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "cycle resume: cwd: %v\n", err)
		return cliExitIO
	}
	rp, rc := pausedCycleOf(root, *cycle, stderr)
	if rc != cliExitOK {
		return rc
	}
	if !*dryRun {
		return runLoop([]string{"--resume", "--project-root", root}, nil, stdout, stderr)
	}
	result := cycleResumeResult{Cycle: rp.CycleID, Phase: rp.Phase, Reason: rp.Reason, State: rp.StatePath, DryRun: true}
	if *asJSON {
		return printVerbJSON("cycle resume", result, stdout, stderr)
	}
	fmt.Fprintf(stdout, "cycle resume: would resume cycle %d at phase %s (paused: %s; state %s)\n", result.Cycle, result.Phase, result.Reason, result.State)
	return cliExitOK
}

func pausedCycleOf(root string, want int, stderr io.Writer) (*core.ResumePoint, int) {
	if live := runlease.LiveRuns(filepath.Join(paths.EvolveDirOf(root), "runs"), time.Now()); len(live) > 0 {
		fmt.Fprintf(stderr, "cycle resume: refused: run %s holds a live lease (%s); a resume runs at a boundary only; stop the wave first: evolve wave stop --wait 10m\n", live[0].Lease.RunID, live[0].Dir)
		return nil, cliExitRefused
	}
	rp, err := cycleResumeLoad(root, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "cycle resume: no paused cycle to resume: %v\n", err)
		return nil, cliExitFailed
	}
	if want > 0 && rp.CycleID != want {
		fmt.Fprintf(stderr, "cycle resume: the paused cycle is %d, not %d; nothing resumed\n", rp.CycleID, want)
		return nil, cliExitInput
	}
	return rp, cliExitOK
}
