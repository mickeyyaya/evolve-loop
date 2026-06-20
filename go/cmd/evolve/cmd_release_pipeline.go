package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/releasepipeline"
)

// runReleasePipeline is `evolve release <target> [flags]` (alias: release-pipeline).
// Mirrors legacy/scripts/release-pipeline.sh:
//
//	0  — published + propagated
//	1  — pre-publish step failed
//	2  — ship.sh failed (nothing pushed)
//	3  — post-publish failed; auto-rollback ran or was skipped
//	10 — invalid arguments
func runReleasePipeline(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var (
		target           string
		dryRun           bool
		noRollback       bool
		skipTests        bool
		strictPass       bool
		requirePreflight bool
		maxPollWaitS     = 300
		fromTag          string
	)

	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, "Usage: evolve release <target-version> [flags]")
			fmt.Fprintln(stdout, "  --dry-run               simulate, no mutations")
			fmt.Fprintln(stdout, "  --no-rollback           do not auto-rollback on post-push failure")
			fmt.Fprintln(stdout, "  --skip-tests            skip preflight gate tests (hot fixes)")
			fmt.Fprintln(stdout, "  --strict-pass           reject WARN verdicts in preflight (treat WARN as FAIL)")
			fmt.Fprintln(stdout, "  --require-preflight     run full-dry-run.sh harness before any step")
			fmt.Fprintln(stdout, "  --max-poll-wait-s N     marketplace propagation deadline (default 300)")
			fmt.Fprintln(stdout, "  --from-tag <tag>        changelog range start (default: previous tag)")
			return 0
		case a == "--dry-run":
			dryRun = true
		case a == "--no-rollback":
			noRollback = true
		case a == "--skip-tests":
			skipTests = true
		case a == "--strict-pass":
			strictPass = true
		case a == "--require-preflight":
			requirePreflight = true
		case a == "--max-poll-wait-s":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "[release-pipeline] --max-poll-wait-s missing value")
				return 10
			}
			n, err := strconv.Atoi(args[i])
			if err != nil || n <= 0 {
				fmt.Fprintln(stderr, "[release-pipeline] --max-poll-wait-s must be integer > 0")
				return 10
			}
			maxPollWaitS = n
		case a == "--from-tag":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "[release-pipeline] --from-tag missing value")
				return 10
			}
			fromTag = args[i]
		case len(a) >= 2 && a[:2] == "--":
			fmt.Fprintf(stderr, "[release-pipeline] unknown flag: %s\n", a)
			return 10
		default:
			if target == "" {
				target = a
			} else {
				fmt.Fprintf(stderr, "[release-pipeline] extra positional arg: %s\n", a)
				return 10
			}
		}
		i++
	}

	if target == "" {
		fmt.Fprintln(stderr, "[release-pipeline] usage: release <target-version> [flags]")
		return 10
	}

	repoRoot := envOrCwd("EVOLVE_PROJECT_ROOT")
	opts := releasepipeline.Options{
		Target:           target,
		RepoRoot:         repoRoot,
		DryRun:           dryRun,
		NoRollback:       noRollback,
		SkipTests:        skipTests,
		StrictPass:       strictPass,
		RequirePreflight: requirePreflight,
		MaxPollWait:      time.Duration(maxPollWaitS) * time.Second,
		FromTag:          fromTag,
		Stderr:           stderr,
	}
	_, err := releasepipeline.Run(opts)
	if err == nil {
		return 0
	}
	if errors.Is(err, releasepipeline.ErrPrePublishFailed) {
		fmt.Fprintf(stderr, "[release-pipeline] FAIL: %v\n", err)
		return 1
	}
	if errors.Is(err, releasepipeline.ErrShipFailed) {
		fmt.Fprintf(stderr, "[release-pipeline] FAIL: %v\n", err)
		return 2
	}
	if errors.Is(err, releasepipeline.ErrPostPublishFailed) {
		fmt.Fprintf(stderr, "[release-pipeline] FAIL: %v\n", err)
		return 3
	}
	fmt.Fprintf(stderr, "[release-pipeline] FAIL: %v\n", err)
	return 1
}
