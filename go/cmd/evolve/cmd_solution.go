package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/solutioncheck"
)

// runSolution — `evolve solution check <solutions/slug> [--project-root DIR]
// [--registry PATH]` (ADR-0099 slice 2): the document deliverable's self-check
// and eval [code] grader. It runs the SAME engine as the build handoff floor
// and the audit gate (internal/solutioncheck) over the contract the registry
// declares, so the three surfaces can never disagree. Exit 0 = well-formed,
// 1 = violations (one per line), 2 = usage or contract not declared.
func runSolution(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 || args[0] != "check" {
		fmt.Fprintln(stderr, "usage: evolve solution check <slug | solutions/slug> [--project-root DIR] [--registry PATH]")
		return 2
	}
	fs := flag.NewFlagSet("solution check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("project-root", "", "repo root holding docs/architecture/phase-registry.json (default: cwd)")
	registry := fs.String("registry", "", "registry path (default: <project-root>/docs/architecture/phase-registry.json)")
	rest, ok := parseInterspersedArgs(fs, args[1:])
	if !ok || len(rest) != 1 {
		fmt.Fprintln(stderr, "usage: evolve solution check <slug | solutions/slug> [--project-root DIR] [--registry PATH]")
		return 2
	}
	if *root == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "evolve solution: %v\n", err)
			return 2
		}
		*root = wd
	}
	if *registry == "" {
		*registry = filepath.Join(*root, "docs", "architecture", "phase-registry.json")
	}
	// The same registry + env the cycle composition root loads, so the CLI can
	// never see a different contract than the floor and the audit gate.
	cfg, warnings := config.Load(*registry, filterEvolveEnv(os.Environ()))
	for _, w := range warnings {
		fmt.Fprintf(stderr, "evolve solution: registry warning [%s]: %s\n", w.Code, w.Message)
	}
	spec, ok := cfg.DocumentSpec()
	if !ok {
		fmt.Fprintf(stderr, "evolve solution: %s declares no config.deliverable_kinds.document\n", *registry)
		return 2
	}
	// The argument is the slug, or a path whose last segment is the slug; the
	// deliverable is always <project-root>/<spec.Root>/<slug> — the ONE
	// location rule Check owns, never re-derived here.
	slug := filepath.Base(filepath.Clean(rest[0]))
	failures := solutioncheck.Check(*root, slug, spec)
	if len(failures) == 0 {
		fmt.Fprintf(stdout, "OK: %s/%s satisfies the document contract\n", spec.Root, slug)
		return 0
	}
	for _, f := range failures {
		fmt.Fprintln(stdout, f.String())
	}
	fmt.Fprintf(stderr, "solution check: %d violation(s)\n", len(failures))
	return 1
}

// parseInterspersedArgs parses flags that may follow positional args
// (`check <dir> --project-root X`), returning the positionals.
func parseInterspersedArgs(fs *flag.FlagSet, args []string) ([]string, bool) {
	var pos []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return nil, false
		}
		rest := fs.Args()
		if len(rest) == 0 {
			break
		}
		pos = append(pos, rest[0])
		args = rest[1:]
	}
	return pos, true
}
