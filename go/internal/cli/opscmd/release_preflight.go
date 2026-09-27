package opscmd

import (
	"errors"
	"fmt"
	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/releasepreflight"
)

func printReleasePreflightUsage(stdout io.Writer) {
	fmt.Fprintln(stdout, "Usage: evolve release-preflight <target-version> [--dry-run] [--skip-tests] [--strict-pass] [--allow-red-ci]")
	fmt.Fprintln(stdout, "5-step gate: clean tree | branch attached | semver bump | recent audit PASS | gate-tests green.")
	fmt.Fprintln(stdout, "Plus the release-commit CI hard-gate: the remote go CI run for HEAD must be conclusion=success.")
	fmt.Fprintln(stdout, "  --strict-pass   reject WARN verdicts (treat WARN as FAIL)")
	fmt.Fprintln(stdout, "  --allow-red-ci  explicit override: proceed on a non-green release-commit CI (logged loudly)")
}

type releasePreflightFlags struct {
	target     string
	dryRun     bool
	skipTests  bool
	strictPass bool
	allowRedCI bool
}

func parseReleasePreflightArgs(args []string, stdout, stderr io.Writer) (f releasePreflightFlags, code int, done bool) {
	for _, a := range args {
		switch {
		case a == "--help" || a == "-h":
			printReleasePreflightUsage(stdout)
			return f, 0, true
		case a == "--dry-run":
			f.dryRun = true
		case a == "--skip-tests":
			f.skipTests = true
		case a == "--strict-pass": // flag "strict-pass": reject WARN verdicts
			f.strictPass = true
		case a == "--allow-red-ci": // explicit red-CI override (never silent)
			f.allowRedCI = true
		case len(a) >= 2 && a[:2] == "--":
			fmt.Fprintf(stderr, "[preflight] unknown flag: %s\n", a)
			return f, 10, true
		default:
			if f.target == "" {
				f.target = a
			} else {
				fmt.Fprintf(stderr, "[preflight] extra positional arg: %s\n", a)
				return f, 10, true
			}
		}
	}
	if f.target == "" {
		fmt.Fprintln(stderr, "[preflight] usage: release-preflight <target-version> [--dry-run] [--skip-tests] [--strict-pass]")
		return f, 10, true
	}
	return f, 0, false
}

// runReleasePreflight is `evolve release-preflight <target> [--dry-run]
// [--skip-tests]`. Mirrors legacy/scripts/release/preflight.sh.
//
// Exit codes:
//
//	0  — all checks pass
//	1  — some check failed
//	10 — invalid arguments
func RunReleasePreflight(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	f, code, done := parseReleasePreflightArgs(args, stdout, stderr)
	if done {
		return code
	}

	repoRoot := cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")

	opts := releasepreflight.Options{
		Target:     f.target,
		RepoRoot:   repoRoot,
		DryRun:     f.dryRun,
		SkipTests:  f.skipTests,
		StrictPass: f.strictPass,
		AllowRedCI: f.allowRedCI,
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
