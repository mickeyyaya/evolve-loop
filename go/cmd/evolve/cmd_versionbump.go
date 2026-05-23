package main

import (
	"fmt"
	"io"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/versionbump"
)

// runVersionBump is `evolve version-bump <target> [--dry-run]`.
// Mirrors legacy/scripts/release/version-bump.sh exit codes:
//
//	0  — success or no-op idempotent
//	1  — a write failed
//	10 — invalid arguments
func runVersionBump(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var (
		target string
		dryRun bool
	)
	for _, a := range args {
		switch {
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, "Usage: evolve version-bump <target-version> [--dry-run]")
			fmt.Fprintln(stdout, "Bumps .claude-plugin/*.json, SKILL.md, README.md atomically.")
			return 0
		case a == "--dry-run":
			dryRun = true
		case len(a) >= 2 && a[:2] == "--":
			fmt.Fprintf(stderr, "[version-bump] unknown flag: %s\n", a)
			return 10
		default:
			if target == "" {
				target = a
			} else {
				fmt.Fprintf(stderr, "[version-bump] extra positional arg: %s\n", a)
				return 10
			}
		}
	}
	if target == "" {
		fmt.Fprintln(stderr, "[version-bump] usage: version-bump <target-version> [--dry-run]")
		return 10
	}

	repoRoot := envOrCwd("EVOLVE_PROJECT_ROOT")
	paths := versionbump.DefaultPaths(repoRoot)
	res, err := versionbump.Run(paths, target, dryRun, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "[version-bump] FAIL: %v\n", err)
		return 1
	}
	if len(res.Modified) == 0 {
		fmt.Fprintf(stderr, "[version-bump] no-op: all files already at %s\n", target)
	} else {
		for _, m := range res.Modified {
			fmt.Fprintf(stderr, "[version-bump] OK: bumped %s → %s\n", m, target)
		}
	}
	fmt.Fprint(stdout, res.ResultJSON())
	return 0
}
