package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
)

func runGCCheck(args []string, stdout, stderr io.Writer) int {
	return runGCCheckWith(args, processGetwd, filepath.Abs, stdout, stderr)
}

func runGCCheckWith(args []string, getwd func() (string, error), abs func(string) (string, error), stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve gc check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectRoot := fs.String("project-root", "", "repository root `dir` to check")
	asJSON := fs.Bool("json", false, "print the check report as JSON")
	if err := fs.Parse(args); err != nil {
		return cliExitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "evolve gc check: unexpected argument %q\n", fs.Arg(0))
		return cliExitUsage
	}
	root := *projectRoot
	if root == "" {
		cwd, err := getwd()
		if err != nil {
			fmt.Fprintf(stderr, "evolve gc check: no --project-root and cwd is unreadable: %v\n", err)
			return cliExitFailed
		}
		root = cwd
	}
	r := newGCRun(context.Background(), true, io.Discard, stderr)
	r.abs = abs
	resolved, pol, _, ok := r.loadFullPolicy(root)
	if !ok {
		return cliExitFailed
	}
	gcPol := gcBlock(pol)
	evolveDir := filepath.Join(resolved, ".evolve")
	cc := gc.CheckContext{
		ProjectRoot: resolved,
		EvolveDir:   evolveDir,
		Policy:      gcPol,
		Worktrees:   worktreeGCOptions(resolved, evolveDir, gcPol.Worktrees, io.Discard),
	}
	report, err := gc.RunChecks(context.Background(), cc, gc.DefaultChecks())
	if err != nil {
		fmt.Fprintf(stderr, "evolve gc check: %v\n", err)
		return cliExitFailed
	}
	return printGCCheckReport(report, *asJSON, stdout, stderr)
}

func printGCCheckReport(report gc.CheckReport, asJSON bool, stdout, stderr io.Writer) int {
	if asJSON {
		return printVerbJSON("gc check", report, stdout, stderr)
	}
	for _, c := range report.Checks {
		fmt.Fprintf(stdout, "check %s: %d item(s)\n", c.Name, len(c.Items))
		for _, it := range c.Items {
			fmt.Fprintf(stdout, "  %s %s [rule=%s] (%s)\n", strings.ToUpper(it.Action), it.Subject, it.Rule, it.Evidence)
		}
	}
	return cliExitOK
}
