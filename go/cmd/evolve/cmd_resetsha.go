package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfsha"
	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
)

// See ADR-0065.
func runResetSHA(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve reset-sha", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var projectRoot string
	var operator bool
	fs.StringVar(&projectRoot, "project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	fs.BoolVar(&operator, "operator", false, "authorize the re-pin even when build-commit provenance is unverifiable")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if projectRoot == "" {
		projectRoot = os.Getenv("EVOLVE_PROJECT_ROOT")
	}
	if projectRoot == "" {
		var err error
		if projectRoot, err = os.Getwd(); err != nil {
			fmt.Fprintf(stderr, "evolve reset-sha: cwd: %v\n", err)
			return 1
		}
	}
	// Mirrors runShipCmd: an absolute root keeps this aligned with the
	// audit/commit-gate paths; RepinShipSHA's IsAbs guard is the terminal check.
	absRoot := paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve reset-sha: WARN: %s\n", m)
	})

	runningSHA, err := selfsha.Running()
	if err != nil {
		fmt.Fprintf(stderr, "evolve reset-sha: hash running binary: %v\n", err)
		return 1
	}
	commit := version.Commit()

	prov := func(c string) bool {
		if c == "" {
			return false
		}
		return exec.Command("git", "-C", absRoot, "merge-base", "--is-ancestor", c, "HEAD").Run() == nil
	}

	statePath := filepath.Join(absRoot, ".evolve", "state.json")
	res, err := phaseintegrity.RepinShipSHA(statePath, runningSHA, commit, "", prov, operator)
	if err != nil {
		fmt.Fprintf(stderr, "evolve reset-sha: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "reset-sha: re-pinned expected_ship_sha %.12s -> %.12s (authorized: %s)\n",
		res.OldSHA, res.NewSHA, res.Authorized)
	return 0
}
