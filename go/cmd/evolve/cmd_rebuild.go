package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfsha"
)

var (
	rebuildBuildFn        = defaultBootRefreshRebuild
	rebuildLiveRunsFn     = runlease.LiveRuns
	rebuildRepinFn        = phaseintegrity.RepinShipSHA
	rebuildProvenanceFn   = defaultShipRepinProvenance
	rebuildAbsoluteRootFn = paths.AbsoluteRoot
	rebuildGetenvFn       = os.Getenv
)

func resolveRebuildProjectRoot(projectRoot string, stderr io.Writer) (string, error) {
	if projectRoot == "" {
		projectRoot = rebuildGetenvFn("EVOLVE_PROJECT_ROOT")
	}
	if projectRoot == "" {
		cwd, err := processGetwd()
		if err != nil {
			return "", fmt.Errorf("cwd: %w", err)
		}
		projectRoot = cwd
	}
	return rebuildAbsoluteRootFn("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve rebuild: WARN: %s\n", m)
	}), nil
}

func handleRebuildRepin(absRoot, oldSHA, newSHA string, stdout, stderr io.Writer) int {
	commit, prov := rebuildProvenanceFn(absRoot)
	statePath := filepath.Join(absRoot, ".evolve", "state.json")
	res, err := rebuildRepinFn(statePath, newSHA, commit, "", prov, false)
	if err != nil {
		fmt.Fprintf(stderr, "evolve rebuild: repin: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "rebuild: %s -> %s\n", oldSHA, newSHA)
	fmt.Fprintf(stdout, "rebuild: re-pinned expected_ship_sha %.12s -> %.12s (authorized: %s)\n",
		res.OldSHA, res.NewSHA, res.Authorized)
	return 0
}

func executeRebuild(absRoot string, stderr io.Writer) int {
	tail := &stderrTail{}
	if err := rebuildBuildFn(absRoot, tail); err != nil {
		for _, line := range tail.Lines() {
			fmt.Fprintln(stderr, line)
		}
		fmt.Fprintf(stderr, "evolve rebuild: build failed: %v\n", err)
		return 2
	}
	return 0
}

func runRebuild(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve rebuild", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var projectRoot string
	var repin bool
	fs.StringVar(&projectRoot, "project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	fs.BoolVar(&repin, "repin", false, "re-pin the ship-gate binary SHA to the rebuilt binary")
	if err := fs.Parse(args); err != nil || len(fs.Args()) > 0 {
		return 2
	}
	absRoot, err := resolveRebuildProjectRoot(projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve rebuild: %v\n", err)
		return 2
	}
	runsDir := filepath.Join(absRoot, ".evolve", "runs")
	if live := rebuildLiveRunsFn(runsDir, time.Now()); len(live) > 0 {
		fmt.Fprintf(stderr, "evolve rebuild: live run lease active in %s (run_id: %s, pid: %d)\n",
			live[0].Dir, live[0].Lease.RunID, live[0].Lease.OwnerPID)
		return 1
	}
	binPath := filepath.Join(absRoot, "go", "bin", "evolve")
	oldSHA, err := selfsha.Of(binPath)
	if err != nil {
		oldSHA = "absent"
	}
	if code := executeRebuild(absRoot, stderr); code != 0 {
		return code
	}
	newSHA, err := selfsha.Of(binPath)
	if err != nil {
		fmt.Fprintf(stderr, "evolve rebuild: hash rebuilt binary: %v\n", err)
		return 2
	}
	if repin {
		return handleRebuildRepin(absRoot, oldSHA, newSHA, stdout, stderr)
	}
	fmt.Fprintf(stdout, "rebuild: %s -> %s\n", oldSHA, newSHA)
	return 0
}
