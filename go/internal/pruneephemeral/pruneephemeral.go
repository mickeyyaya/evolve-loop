// Package pruneephemeral ports legacy/scripts/observability/prune-ephemeral.sh.
//
// TTL retention for phase-tracker ephemeral data. Idempotent. Safe to run
// repeatedly. Safe to run while a cycle is in progress (recently-modified
// files are protected by the mtime filter).
//
// Retention policy (defaults; env-overridable):
//
//	.evolve/runs/cycle-*/.ephemeral/   → 7 days  (EVOLVE_TRACKER_TTL_DAYS)
//	.evolve/dispatch-logs/batch-*.log  → 30 days (EVOLVE_DISPATCH_LOG_TTL_DAYS)
//	.evolve/runs/cycle-*/*.md          → never pruned
//	.evolve/runs/cycle-*/*.json        → never pruned
//	.evolve/ledger.jsonl               → never pruned (append-only, tamper-evident)
//
// Exit codes (mapped by cmd layer):
//
//	0  — success (whether anything was pruned or not)
//	10 — bad arguments
package pruneephemeral

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Options drives a Run() invocation.
type Options struct {
	ProjectRoot     string        // required
	RunsDir         string        // optional override; defaults to <ProjectRoot>/.evolve/runs
	DispatchLogsDir string        // optional override; defaults to <ProjectRoot>/.evolve/dispatch-logs
	TrackerTTL      time.Duration // default 7d
	DispatchLogTTL  time.Duration // default 30d
	DryRun          bool
	Quiet           bool
	Stderr          io.Writer

	Now func() time.Time
}

// Result summarizes how many entries were pruned (or would be in dry-run).
type Result struct {
	EphemeralPruned int
	LogFilesPruned  int
	EphemeralPaths  []string // populated in dry-run for auditing
	LogPaths        []string
}

// Run executes the prune pipeline.
func Run(opts Options) (Result, error) {
	res := Result{EphemeralPaths: []string{}, LogPaths: []string{}}

	if opts.ProjectRoot == "" {
		return res, fmt.Errorf("pruneephemeral: ProjectRoot required")
	}
	runsDir := opts.RunsDir
	if runsDir == "" {
		runsDir = filepath.Join(opts.ProjectRoot, ".evolve", "runs")
	}
	dispatchLogsDir := opts.DispatchLogsDir
	if dispatchLogsDir == "" {
		dispatchLogsDir = filepath.Join(opts.ProjectRoot, ".evolve", "dispatch-logs")
	}
	if opts.TrackerTTL <= 0 {
		opts.TrackerTTL = 7 * 24 * time.Hour
	}
	if opts.DispatchLogTTL <= 0 {
		opts.DispatchLogTTL = 30 * 24 * time.Hour
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	logw := opts.Stderr
	if logw == nil {
		logw = io.Discard
	}
	logf := func(format string, args ...any) {
		if opts.Quiet {
			return
		}
		fmt.Fprintf(logw, "[prune-ephemeral] "+format+"\n", args...)
	}

	cutoffTracker := now().Add(-opts.TrackerTTL)
	cutoffLog := now().Add(-opts.DispatchLogTTL)

	// Phase 1: .ephemeral/ subtrees under cycle-N/ (maxdepth 3).
	if info, err := os.Stat(runsDir); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(runsDir)
		for _, cycleEntry := range entries {
			if !cycleEntry.IsDir() {
				continue
			}
			cycleDir := filepath.Join(runsDir, cycleEntry.Name())
			ephemeralDir := filepath.Join(cycleDir, ".ephemeral")
			info, err := os.Stat(ephemeralDir)
			if err != nil || !info.IsDir() {
				continue
			}
			if !info.ModTime().Before(cutoffTracker) {
				continue
			}
			if opts.DryRun {
				logf("DRY-RUN would remove %s", ephemeralDir)
			} else {
				if err := os.RemoveAll(ephemeralDir); err == nil {
					logf("removed %s", ephemeralDir)
				} else {
					logf("WARN: failed to remove %s: %v", ephemeralDir, err)
					continue
				}
			}
			res.EphemeralPaths = append(res.EphemeralPaths, ephemeralDir)
			res.EphemeralPruned++
		}
	}

	// Phase 2: batch-*.log files under .evolve/dispatch-logs/ (maxdepth 1).
	if info, err := os.Stat(dispatchLogsDir); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(dispatchLogsDir)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasPrefix(name, "batch-") || !strings.HasSuffix(name, ".log") {
				continue
			}
			path := filepath.Join(dispatchLogsDir, name)
			info, err := os.Stat(path)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			if !info.ModTime().Before(cutoffLog) {
				continue
			}
			if opts.DryRun {
				logf("DRY-RUN would remove %s", path)
			} else {
				if err := os.Remove(path); err == nil {
					logf("removed %s", path)
				} else {
					logf("WARN: failed to remove %s: %v", path, err)
					continue
				}
			}
			res.LogPaths = append(res.LogPaths, path)
			res.LogFilesPruned++
		}
	}

	// Summary.
	if opts.Quiet {
		// Quiet mode: only emit one line if anything was actually pruned.
		if res.EphemeralPruned > 0 || res.LogFilesPruned > 0 {
			fmt.Fprintf(logw, "[prune-ephemeral] pruned %d ephemeral dirs, %d log files\n",
				res.EphemeralPruned, res.LogFilesPruned)
		}
	} else {
		logf("summary: ephemeral=%d log_files=%d (dry_run=%v, ttl_days=%d / %d)",
			res.EphemeralPruned, res.LogFilesPruned, opts.DryRun,
			int(opts.TrackerTTL.Hours()/24), int(opts.DispatchLogTTL.Hours()/24))
	}
	return res, nil
}
