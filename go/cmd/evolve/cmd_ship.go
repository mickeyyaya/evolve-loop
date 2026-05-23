package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
)

// runShipCmd implements `evolve ship` — the native Go replacement for
// `bash legacy/scripts/lifecycle/ship.sh`. Drop-in for hook chains, operator use,
// and the release-pipeline.
//
// Usage:
//
//	evolve ship "<commit-message>"                   # --class cycle (default)
//	evolve ship --class manual "<commit-message>"
//	evolve ship --class release "<commit-message>"
//	evolve ship --class trivial "<commit-message>"
//	evolve ship --dry-run "<commit-message>"
//
// Exit codes mirror ship.sh:
//
//	0   — shipped
//	1   — runtime failure (bad args, missing binary, git fail)
//	2   — integrity failure (audit binding, SHA-pin tamper, manual refused)
//	127 — required binary missing
func runShipCmd(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve ship", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var class string
	var dryRun bool
	var projectRoot string
	var pluginRoot string
	fs.StringVar(&class, "class", "cycle", "commit class (cycle|manual|release|trivial)")
	fs.BoolVar(&dryRun, "dry-run", false, "run all read-only checks but skip commit/push/release")
	fs.StringVar(&projectRoot, "project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	fs.StringVar(&pluginRoot, "plugin-root", "", "plugin root (default: $EVOLVE_PLUGIN_ROOT or project root)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, `evolve ship: usage: evolve ship [--class cycle|manual|release|trivial] [--dry-run] "<commit-message>"`)
		return 1
	}
	if fs.NArg() > 1 {
		fmt.Fprintf(stderr, "evolve ship: extra positional args (only one commit message expected): %v\n", fs.Args()[1:])
		return 1
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
	if pluginRoot == "" {
		pluginRoot = os.Getenv("EVOLVE_PLUGIN_ROOT")
	}

	opts := ship.Options{
		Class:         ship.Class(class),
		CommitMessage: fs.Arg(0),
		DryRun:        dryRun,
		ProjectRoot:   projectRoot,
		PluginRoot:    pluginRoot,
		Stdin:         stdin,
		Stdout:        stdout,
		Stderr:        stderr,
	}

	res, err := ship.Run(context.Background(), opts)

	// Emit logs to stderr so callers can grep them (matches ship.sh's `log` → stderr).
	for _, line := range res.Logs {
		fmt.Fprintln(stderr, line)
	}

	if err != nil {
		// Translate error categories to exit codes.
		switch err.(type) {
		case *ship.IntegrityError:
			return int(ship.ExitIntegrity)
		}
		// Other errors from validation (e.g. invalid class) → runtime failure.
		if strings.Contains(err.Error(), "invalid --class") || strings.Contains(err.Error(), "commit message required") {
			return int(ship.ExitFailure)
		}
		return int(ship.ExitFailure)
	}
	return int(res.ExitCode)
}
