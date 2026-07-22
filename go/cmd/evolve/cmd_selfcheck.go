package main

// cmd_selfcheck.go — ADR-0076 slice B (green-before-handoff): `evolve
// selfcheck build` runs the build handoff floor's EXACT deterministic checks
// (core.DefaultBuildFloorChecks) as an in-session pre-flight, so the builder
// fixes findings inside its own loop and budget instead of post-hoc
// correction windows. The floor itself is unchanged — it remains the
// trust-boundary backstop; this is the same check moved to where fixing is
// cheap. Exit codes: 0 green, 1 findings, 2 usage.

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// buildFloorChecksFn is the DI seam holding the floor's check function —
// tests inject stubs; the wiring pin holds the default to the REAL floor so
// the CLI and the reviewer can never drift apart.
var buildFloorChecksFn = core.DefaultBuildFloorChecks

func runSelfcheck(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 || args[0] != "build" {
		fmt.Fprintln(stderr, "usage: evolve selfcheck build [--worktree DIR] — run the build handoff floor's exact checks as a pre-flight; iterate until GREEN before declaring done (ADR-0076)")
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
	findings := buildFloorChecksFn(context.Background(), core.ReviewInput{Worktree: dir})
	if len(findings) == 0 {
		fmt.Fprintf(stdout, "[selfcheck] GREEN: build handoff floor checks pass for %s — safe to hand off\n", dir)
		return 0
	}
	fmt.Fprintf(stdout, "[selfcheck] %d finding(s) — fix these exactly, then re-run until GREEN:\n", len(findings))
	for _, f := range findings {
		fmt.Fprintf(stdout, "  %s\n", f)
	}
	return 1
}
