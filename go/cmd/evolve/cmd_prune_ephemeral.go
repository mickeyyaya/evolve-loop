package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/pruneephemeral"
)

// runPruneEphemeral is `evolve prune-ephemeral [--dry-run] [--quiet]`.
// Mirrors legacy/scripts/observability/prune-ephemeral.sh exit codes:
//
//	0  — success (whether anything was pruned or not)
//	10 — bad arguments
//
// Env honored (matches bash):
//
//	EVOLVE_TRACKER_TTL_DAYS       (default 7)
//	EVOLVE_DISPATCH_LOG_TTL_DAYS  (default 30)
//	EVOLVE_PROJECT_ROOT           (default: cwd)
func runPruneEphemeral(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var (
		dryRun bool
		quiet  bool
	)
	for _, a := range args {
		switch {
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, "Usage: evolve prune-ephemeral [--dry-run] [--quiet]")
			fmt.Fprintln(stdout, "TTL retention for .evolve/runs/cycle-*/.ephemeral and .evolve/dispatch-logs/batch-*.log")
			fmt.Fprintln(stdout, "Env: EVOLVE_TRACKER_TTL_DAYS (7), EVOLVE_DISPATCH_LOG_TTL_DAYS (30), EVOLVE_PROJECT_ROOT")
			return 0
		case a == "--dry-run":
			dryRun = true
		case a == "--quiet":
			quiet = true
		case len(a) >= 2 && a[:2] == "--":
			fmt.Fprintf(stderr, "[prune-ephemeral] unknown flag: %s\n", a)
			return 10
		default:
			fmt.Fprintf(stderr, "[prune-ephemeral] unexpected arg: %s\n", a)
			return 10
		}
	}

	trackerTTLDays := 7
	if v := os.Getenv("EVOLVE_TRACKER_TTL_DAYS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			fmt.Fprintln(stderr, "[prune-ephemeral] EVOLVE_TRACKER_TTL_DAYS must be integer")
			return 10
		}
		trackerTTLDays = n
	}
	logTTLDays := 30
	if v := os.Getenv("EVOLVE_DISPATCH_LOG_TTL_DAYS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			fmt.Fprintln(stderr, "[prune-ephemeral] EVOLVE_DISPATCH_LOG_TTL_DAYS must be integer")
			return 10
		}
		logTTLDays = n
	}

	projectRoot := envOrCwd("EVOLVE_PROJECT_ROOT")
	_, err := pruneephemeral.Run(pruneephemeral.Options{
		ProjectRoot:    projectRoot,
		TrackerTTL:     time.Duration(trackerTTLDays) * 24 * time.Hour,
		DispatchLogTTL: time.Duration(logTTLDays) * 24 * time.Hour,
		DryRun:         dryRun,
		Quiet:          quiet,
		Stderr:         stderr,
	})
	if err != nil {
		fmt.Fprintf(stderr, "[prune-ephemeral] FAIL: %v\n", err)
		return 1
	}
	return 0
}
