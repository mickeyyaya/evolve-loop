package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var (
	shipRunFn        = ship.Run
	shipAbsoluteRoot = paths.AbsoluteRoot
)

func runShipCmd(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve ship", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var class string
	var dryRun bool
	var bypassCommitGate bool
	var bypassPrefixGate bool
	var pushOnly bool
	var projectRoot string
	var pluginRoot string
	var pr shipPrRequest
	fs.BoolVar(&pr.enabled, "pr", false, "after a successful manual ship, open the pull request of the pushed branch (or report the open one)")
	fs.StringVar(&pr.bodyFile, "pr-body-file", "", "file holding the pull request body (requires --pr)")
	fs.StringVar(&class, "class", "cycle", "commit class (cycle|manual|release|trivial)")
	fs.BoolVar(&pushOnly, "push-only", false, "push an already-committed, provenance-verified ahead set (the sanctioned completion after GIT_PUSH_REJECTED + sync-main); commits nothing")
	fs.BoolVar(&dryRun, "dry-run", false, "run all read-only checks but skip commit/push/release")
	fs.BoolVar(&bypassCommitGate, "bypass-commit-gate", false, "emergency: skip manual commit-gate attestation")
	fs.BoolVar(&bypassPrefixGate, "bypass-prefix-gate", false, "emergency: skip commit-prefix gate for manual class")
	fs.StringVar(&projectRoot, "project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	fs.StringVar(&pluginRoot, "plugin-root", "", "plugin root (default: $EVOLVE_PLUGIN_ROOT or project root)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if pushOnly {
		if fs.NArg() > 0 {
			fmt.Fprintln(stderr, "evolve ship: --push-only takes no commit message (it commits nothing)")
			return 1
		}
	} else {
		if fs.NArg() < 1 {
			fmt.Fprintln(stderr, `evolve ship: usage: evolve ship [--class cycle|manual|release|trivial] [--dry-run] "<commit-message>" | evolve ship --push-only`)
			return 1
		}
		if fs.NArg() > 1 {
			fmt.Fprintf(stderr, "evolve ship: extra positional args (only one commit message expected): %v\n", fs.Args()[1:])
			return 1
		}
	}

	projectRoot, rc := resolveShipRoot(projectRoot, stderr)
	if rc != 0 {
		return rc
	}
	if pluginRoot == "" {
		pluginRoot = os.Getenv("EVOLVE_PLUGIN_ROOT")
	}
	prBranch, prRc := resolveShipPrBranch(pr, class, pushOnly, projectRoot, stderr)
	if prRc != 0 {
		return prRc
	}
	signals, closeSignals := shipRootSignals(projectRoot, stderr)
	defer closeSignals()

	opts := ship.Options{
		Class:            ship.Class(class),
		CommitMessage:    fs.Arg(0),
		DryRun:           dryRun,
		BypassCommitGate: bypassCommitGate,
		BypassPrefixGate: bypassPrefixGate,
		PushOnly:         pushOnly,
		ProjectRoot:      projectRoot,
		PluginRoot:       pluginRoot,
		Stdin:            stdin,
		Stdout:           stdout,
		Stderr:           stderr,
		Signals:          signals,
	}

	res, err := shipRunFn(context.Background(), opts)

	for _, line := range res.Logs {
		fmt.Fprintln(stderr, line)
	}

	if err != nil {
		if res.ExitCode != ship.ExitOK {
			return int(res.ExitCode)
		}
		return int(ship.ExitFailure)
	}
	if pr.enabled && !dryRun && res.ExitCode == ship.ExitOK {
		return openShipPr(pr, projectRoot, prBranch, fs.Arg(0), stdout, stderr)
	}
	return int(res.ExitCode)
}

func shipRootSignals(projectRoot string, stderr io.Writer) (*signalcenter.Center, func()) {
	return newRoleSignalCenter(roleShip, projectRoot, paths.EvolveDirOf(projectRoot), stderr)
}

func resolveShipRoot(projectRoot string, stderr io.Writer) (string, int) {
	if projectRoot == "" {
		projectRoot = os.Getenv("EVOLVE_PROJECT_ROOT")
	}
	if projectRoot == "" {
		var err error
		projectRoot, err = processGetwd()
		if err != nil {
			fmt.Fprintf(stderr, "evolve ship: cwd: %v\n", err)
			return "", 1
		}
	}
	projectRoot = shipAbsoluteRoot("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve ship: WARN: %s\n", m)
	})
	return projectRoot, 0
}
