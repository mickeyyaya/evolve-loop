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

	if projectRoot == "" {
		projectRoot = os.Getenv("EVOLVE_PROJECT_ROOT")
	}
	if projectRoot == "" {
		var err error
		projectRoot, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "evolve ship: cwd: %v\n", err)
			return 1
		}
	}
	projectRoot = paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve ship: WARN: %s\n", m)
	})
	if pluginRoot == "" {
		pluginRoot = os.Getenv("EVOLVE_PLUGIN_ROOT")
	}
	signals := shipRootSignals(projectRoot, stderr)
	defer signals.Flush()

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

	res, err := ship.Run(context.Background(), opts)

	for _, line := range res.Logs {
		fmt.Fprintln(stderr, line)
	}

	if err != nil {
		// finalize() classifies err into ExitCode (integrity → ExitIntegrity, else
		// ExitFailure); early-validation errors bypass it, leaving ExitCode==ExitOK.
		if res.ExitCode != ship.ExitOK {
			return int(res.ExitCode)
		}
		return int(ship.ExitFailure)
	}
	return int(res.ExitCode)
}

func shipRootSignals(projectRoot string, stderr io.Writer) *signalcenter.Center {
	return newRootSignalCenter(projectRoot, paths.EvolveDirOf(projectRoot), stderr)
}
