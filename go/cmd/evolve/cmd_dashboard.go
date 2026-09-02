package main

// cmd_dashboard.go — `evolve dashboard [--project-root P] [--addr A] [--snapshot]`
// serves the read-only live pipeline dashboard (internal/dashboard, ADR-0095):
// loop status, inbox, per-cycle progress, what went wrong, ship-rate trend.
// Pure reader: no state, ledger, inbox, or registry mutation; never takes the
// loop's flock sidecars — safe to run beside a live loop. --snapshot prints the
// JSON snapshot once to stdout and exits (scripting / smoke checks).

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/dashboard"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

// dashboardServe is the seam between flag parsing and the long-running server
// so the command's wiring is testable without binding a port. At runtime it is
// exactly dashboard.New(...).ListenAndServe.
var dashboardServe = func(ctx context.Context, root, addr string) error {
	return dashboard.New(root, dashboard.Options{}).ListenAndServe(ctx, addr)
}

func runDashboard(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dashboard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("project-root", "", "project root (default: EVOLVE_PROJECT_ROOT or cwd)")
	addr := fs.String("addr", dashboard.DefaultAddr, "listen address; loopback by default — the page renders agent-authored text and has no auth")
	snapshotOnly := fs.Bool("snapshot", false, "print the JSON snapshot to stdout and exit instead of serving")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	pr := dashboardProjectRoot(*root, stderr)
	if *snapshotOnly {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(dashboard.Collect(pr, time.Now())); err != nil {
			fmt.Fprintf(stderr, "evolve dashboard: %v\n", err)
			return 1
		}
		return 0
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Fprintf(stderr, "evolve dashboard: serving %s on http://%s (Ctrl-C to stop)\n", pr, *addr)
	if err := dashboardServe(ctx, pr, *addr); err != nil {
		fmt.Fprintf(stderr, "evolve dashboard: %v\n", err)
		return 1
	}
	return 0
}

// dashboardProjectRoot resolves flag → EVOLVE_PROJECT_ROOT → cwd through the
// shared cmdutil rule, then makes the flag value absolute (paths.AbsoluteRoot:
// a relative root is the cycle-119 defect class — the server and the artifacts
// it names must agree on one location).
func dashboardProjectRoot(flagRoot string, stderr io.Writer) string {
	if flagRoot == "" {
		return cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")
	}
	return paths.AbsoluteRoot("--project-root", flagRoot, func(msg string) { fmt.Fprintln(stderr, "evolve dashboard: WARN:", msg) })
}
