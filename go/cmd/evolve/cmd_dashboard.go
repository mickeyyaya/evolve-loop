package main

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
		if err := enc.Encode(dashboard.New(pr, dashboard.Options{}).Snapshot(time.Now())); err != nil {
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

func dashboardProjectRoot(flagRoot string, stderr io.Writer) string {
	if flagRoot == "" {
		return cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")
	}
	return paths.AbsoluteRoot("--project-root", flagRoot, func(msg string) { fmt.Fprintln(stderr, "evolve dashboard: WARN:", msg) })
}
