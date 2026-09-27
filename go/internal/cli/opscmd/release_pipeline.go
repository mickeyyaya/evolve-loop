package opscmd

import (
	"errors"
	"fmt"
	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"io"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/releasepipeline"
)

func printReleasePipelineUsage(stdout io.Writer) {
	fmt.Fprintln(stdout, "Usage: evolve release <target-version> [flags]")
	fmt.Fprintln(stdout, "  --dry-run               simulate, no mutations")
	fmt.Fprintln(stdout, "  --no-rollback           do not auto-rollback on post-push failure")
	fmt.Fprintln(stdout, "  --skip-tests            skip preflight gate tests (hot fixes)")
	fmt.Fprintln(stdout, "  --strict-pass           reject WARN verdicts in preflight (treat WARN as FAIL)")
	fmt.Fprintln(stdout, "  --require-preflight     run full-dry-run.sh harness before any step")
	fmt.Fprintln(stdout, "  --max-poll-wait-s N     marketplace propagation deadline (default 300)")
	fmt.Fprintln(stdout, "  --from-tag <tag>        changelog range start (default: previous tag)")
}

type releasePipelineFlags struct {
	target           string
	dryRun           bool
	noRollback       bool
	skipTests        bool
	strictPass       bool
	requirePreflight bool
	maxPollWaitS     int
	fromTag          string
}

func releasePipelineFlagCase(f *releasePipelineFlags, a string, args []string, i *int, stderr io.Writer) (handled, ok bool) {
	switch {
	case a == "--dry-run":
		f.dryRun = true
	case a == "--no-rollback":
		f.noRollback = true
	case a == "--skip-tests":
		f.skipTests = true
	case a == "--strict-pass":
		f.strictPass = true
	case a == "--require-preflight":
		f.requirePreflight = true
	case a == "--max-poll-wait-s":
		*i++
		if *i >= len(args) {
			fmt.Fprintln(stderr, "[release-pipeline] --max-poll-wait-s missing value")
			return true, false
		}
		n, err := strconv.Atoi(args[*i])
		if err != nil || n <= 0 {
			fmt.Fprintln(stderr, "[release-pipeline] --max-poll-wait-s must be integer > 0")
			return true, false
		}
		f.maxPollWaitS = n
	case a == "--from-tag":
		*i++
		if *i >= len(args) {
			fmt.Fprintln(stderr, "[release-pipeline] --from-tag missing value")
			return true, false
		}
		f.fromTag = args[*i]
	case len(a) >= 2 && a[:2] == "--":
		fmt.Fprintf(stderr, "[release-pipeline] unknown flag: %s\n", a)
		return true, false
	default:
		return false, true
	}
	return true, true
}

func parseReleasePipelineArgs(args []string, stdout, stderr io.Writer) (f releasePipelineFlags, code int, done bool) {
	f.maxPollWaitS = 300

	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--help" || a == "-h" {
			printReleasePipelineUsage(stdout)
			return f, 0, true
		}
		if handled, ok := releasePipelineFlagCase(&f, a, args, &i, stderr); handled {
			if !ok {
				return f, 10, true
			}
			continue
		}
		if f.target == "" {
			f.target = a
		} else {
			fmt.Fprintf(stderr, "[release-pipeline] extra positional arg: %s\n", a)
			return f, 10, true
		}
	}

	if f.target == "" {
		fmt.Fprintln(stderr, "[release-pipeline] usage: release <target-version> [flags]")
		return f, 10, true
	}
	return f, 0, false
}

func releasePipelineExitCode(err error, stderr io.Writer) int {
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

// runReleasePipeline is `evolve release <target> [flags]` (alias: release-pipeline).
// Mirrors legacy/scripts/release-pipeline.sh:
//
//	0  — published + propagated
//	1  — pre-publish step failed
//	2  — ship.sh failed (nothing pushed)
//	3  — post-publish failed; auto-rollback ran or was skipped
//	10 — invalid arguments
func RunReleasePipeline(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	f, code, done := parseReleasePipelineArgs(args, stdout, stderr)
	if done {
		return code
	}

	repoRoot := cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")
	opts := releasepipeline.Options{
		Target:           f.target,
		RepoRoot:         repoRoot,
		DryRun:           f.dryRun,
		NoRollback:       f.noRollback,
		SkipTests:        f.skipTests,
		StrictPass:       f.strictPass,
		RequirePreflight: f.requirePreflight,
		MaxPollWait:      time.Duration(f.maxPollWaitS) * time.Second,
		FromTag:          f.fromTag,
		Stderr:           stderr,
	}
	_, err := releasepipeline.Run(opts)
	return releasePipelineExitCode(err, stderr)
}
