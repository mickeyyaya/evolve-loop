// Package pruneephemeral applies TTL retention to the ephemeral data of the phase tracker.
// See docs/architecture/packages/internal-pruneephemeral.md.
package pruneephemeral

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Options struct {
	ProjectRoot     string
	RunsDir         string
	DispatchLogsDir string
	TrackerTTL      time.Duration
	DispatchLogTTL  time.Duration
	DryRun          bool
	Quiet           bool
	Stderr          io.Writer

	Now func() time.Time
}

type Result struct {
	EphemeralPruned int
	LogFilesPruned  int
	EphemeralPaths  []string
	LogPaths        []string
}

func Run(opts Options) (Result, error) {
	res := Result{EphemeralPaths: []string{}, LogPaths: []string{}}

	if opts.ProjectRoot == "" {
		return res, fmt.Errorf("pruneephemeral: ProjectRoot required")
	}
	runsDir, dispatchLogsDir := resolvePruneDirs(opts)
	opts.TrackerTTL = defaultDuration(opts.TrackerTTL, 7*24*time.Hour)
	opts.DispatchLogTTL = defaultDuration(opts.DispatchLogTTL, 30*24*time.Hour)
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	logw := opts.Stderr
	if logw == nil {
		logw = io.Discard
	}
	logf := newPruneLogger(logw, opts.Quiet)

	cutoffTracker := now().Add(-opts.TrackerTTL)
	cutoffLog := now().Add(-opts.DispatchLogTTL)

	res.EphemeralPaths, res.EphemeralPruned = pruneEphemeralDirs(runsDir, cutoffTracker, opts.DryRun, logf)
	res.LogPaths, res.LogFilesPruned = pruneDispatchLogs(dispatchLogsDir, cutoffLog, opts.DryRun, logf)

	logPruneSummary(opts, res, logw, logf)
	return res, nil
}

func resolvePruneDirs(opts Options) (runsDir, dispatchLogsDir string) {
	runsDir = opts.RunsDir
	if runsDir == "" {
		runsDir = filepath.Join(opts.ProjectRoot, ".evolve", "runs")
	}
	dispatchLogsDir = opts.DispatchLogsDir
	if dispatchLogsDir == "" {
		dispatchLogsDir = filepath.Join(opts.ProjectRoot, ".evolve", "dispatch-logs")
	}
	return runsDir, dispatchLogsDir
}

func defaultDuration(d, fallback time.Duration) time.Duration {
	if d <= 0 {
		return fallback
	}
	return d
}

func newPruneLogger(logw io.Writer, quiet bool) func(string, ...any) {
	return func(format string, args ...any) {
		if quiet {
			return
		}
		fmt.Fprintf(logw, "[prune-ephemeral] "+format+"\n", args...)
	}
}

func pruneEphemeralDirs(runsDir string, cutoff time.Time, dryRun bool, logf func(string, ...any)) ([]string, int) {
	paths := []string{}
	count := 0
	info, err := os.Stat(runsDir)
	if err != nil || !info.IsDir() {
		return paths, count
	}
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
		if !info.ModTime().Before(cutoff) {
			continue
		}
		if dryRun {
			logf("DRY-RUN would remove %s", ephemeralDir)
		} else {
			if err := os.RemoveAll(ephemeralDir); err == nil {
				logf("removed %s", ephemeralDir)
			} else {
				logf("WARN: failed to remove %s: %v", ephemeralDir, err)
				continue
			}
		}
		paths = append(paths, ephemeralDir)
		count++
	}
	return paths, count
}

func pruneDispatchLogs(dispatchLogsDir string, cutoff time.Time, dryRun bool, logf func(string, ...any)) ([]string, int) {
	paths := []string{}
	count := 0
	info, err := os.Stat(dispatchLogsDir)
	if err != nil || !info.IsDir() {
		return paths, count
	}
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
		if !info.ModTime().Before(cutoff) {
			continue
		}
		if dryRun {
			logf("DRY-RUN would remove %s", path)
		} else {
			if err := os.Remove(path); err == nil {
				logf("removed %s", path)
			} else {
				logf("WARN: failed to remove %s: %v", path, err)
				continue
			}
		}
		paths = append(paths, path)
		count++
	}
	return paths, count
}

func logPruneSummary(opts Options, res Result, logw io.Writer, logf func(string, ...any)) {
	if opts.Quiet {
		if res.EphemeralPruned > 0 || res.LogFilesPruned > 0 {
			fmt.Fprintf(logw, "[prune-ephemeral] pruned %d ephemeral dirs, %d log files\n",
				res.EphemeralPruned, res.LogFilesPruned)
		}
		return
	}
	logf("summary: ephemeral=%d log_files=%d (dry_run=%v, ttl_days=%d / %d)",
		res.EphemeralPruned, res.LogFilesPruned, opts.DryRun,
		int(opts.TrackerTTL.Hours()/24), int(opts.DispatchLogTTL.Hours()/24))
}
