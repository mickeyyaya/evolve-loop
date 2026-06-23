package main

import (
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/pruneephemeral"
)

// runPruneEphemeral is `evolve prune-ephemeral [--dry-run] [--quiet]
//
//	[--tracker-ttl-days N] [--dispatch-log-ttl-days N]`.
//
// Mirrors legacy/scripts/observability/prune-ephemeral.sh exit codes:
//
//	0  — success (whether anything was pruned or not)
//	10 — bad arguments
//
// Env honored:
//
//	EVOLVE_PROJECT_ROOT  (default: cwd)
func runPruneEphemeral(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("prune-ephemeral", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		dryRun         bool
		quiet          bool
		trackerTTLDays int
		logTTLDays     int
	)
	fs.BoolVar(&dryRun, "dry-run", false, "dry run — show what would be pruned without deleting")
	fs.BoolVar(&quiet, "quiet", false, "suppress progress output")
	fs.IntVar(&trackerTTLDays, "tracker-ttl-days", 7, "tracker retention days")
	fs.IntVar(&logTTLDays, "dispatch-log-ttl-days", 30, "dispatch log retention days")
	// intercept -h/--help before fs.Parse to write to stdout
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "Usage: evolve prune-ephemeral [--dry-run] [--quiet] [--tracker-ttl-days N] [--dispatch-log-ttl-days N]")
			fmt.Fprintln(stdout, "TTL retention for .evolve/runs/cycle-*/.ephemeral and .evolve/dispatch-logs/batch-*.log")
			fmt.Fprintln(stdout, "Env: EVOLVE_PROJECT_ROOT")
			return 0
		}
	}
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "[prune-ephemeral] unexpected arg: %s\n", fs.Arg(0))
		return 10
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
