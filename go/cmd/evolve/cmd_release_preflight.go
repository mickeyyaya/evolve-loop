package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/releasepreflight"
)

// runReleasePreflight is `evolve release-preflight <target> [--dry-run]
// [--skip-tests]`. Mirrors legacy/scripts/release/preflight.sh.
//
// Exit codes:
//
//	0  — all checks pass
//	1  — some check failed
//	10 — invalid arguments
func runReleasePreflight(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var (
		target    string
		dryRun    bool
		skipTests bool
	)
	for _, a := range args {
		switch {
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, "Usage: evolve release-preflight <target-version> [--dry-run] [--skip-tests]")
			fmt.Fprintln(stdout, "5-step gate: clean tree | branch attached | semver bump | recent audit PASS | gate-tests green.")
			fmt.Fprintln(stdout, "Env: EVOLVE_RELEASE_STRICT_PASS=1 to reject WARN verdicts.")
			return 0
		case a == "--dry-run":
			dryRun = true
		case a == "--skip-tests":
			skipTests = true
		case len(a) >= 2 && a[:2] == "--":
			fmt.Fprintf(stderr, "[preflight] unknown flag: %s\n", a)
			return 10
		default:
			if target == "" {
				target = a
			} else {
				fmt.Fprintf(stderr, "[preflight] extra positional arg: %s\n", a)
				return 10
			}
		}
	}
	if target == "" {
		fmt.Fprintln(stderr, "[preflight] usage: release-preflight <target-version> [--dry-run] [--skip-tests]")
		return 10
	}

	repoRoot := envOrCwd("EVOLVE_PROJECT_ROOT")
	strictPass := os.Getenv("EVOLVE_RELEASE_STRICT_PASS") == "1"

	opts := releasepreflight.Options{
		Target:     target,
		RepoRoot:   repoRoot,
		DryRun:     dryRun,
		SkipTests:  skipTests,
		StrictPass: strictPass,
		Stderr:     stderr,
	}
	_, err := releasepreflight.Run(opts)
	if err == nil {
		return 0
	}
	if errors.Is(err, releasepreflight.ErrCheckFailed) {
		fmt.Fprintf(stderr, "[preflight] FAIL: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "[preflight] FAIL: %v\n", err)
	return 1
}
