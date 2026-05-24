package pruneephemeral

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// makeRepo sets up a fake .evolve/runs/cycle-N/.ephemeral + .evolve/dispatch-logs/
// layout with controllable mtimes for the .ephemeral dirs and log files.
//
// Returns the repo root.
func makeRepo(t *testing.T, cycles []cycleSpec, logs []logSpec) string {
	t.Helper()
	d := t.TempDir()
	runsDir := filepath.Join(d, ".evolve", "runs")
	for _, c := range cycles {
		dir := filepath.Join(runsDir, c.name, ".ephemeral")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		// Drop a file inside so the dir is non-empty.
		if err := os.WriteFile(filepath.Join(dir, "data.json"), []byte("{}"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		if !c.modTime.IsZero() {
			if err := os.Chtimes(dir, c.modTime, c.modTime); err != nil {
				t.Fatalf("chtime %s: %v", dir, err)
			}
		}
	}
	logsDir := filepath.Join(d, ".evolve", "dispatch-logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	for _, l := range logs {
		path := filepath.Join(logsDir, l.name)
		if err := os.WriteFile(path, []byte("log"), 0o644); err != nil {
			t.Fatalf("write log: %v", err)
		}
		if !l.modTime.IsZero() {
			if err := os.Chtimes(path, l.modTime, l.modTime); err != nil {
				t.Fatalf("chtime log: %v", err)
			}
		}
	}
	return d
}

type cycleSpec struct {
	name    string
	modTime time.Time
}

type logSpec struct {
	name    string
	modTime time.Time
}

// === Test 1: stale ephemerals get pruned; fresh ones survive ===============
func TestRun_PrunesOldEphemerals(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-10 * 24 * time.Hour) // 10 days old
	fresh := now.Add(-2 * 24 * time.Hour)  // 2 days old

	repo := makeRepo(t,
		[]cycleSpec{
			{"cycle-1", stale},
			{"cycle-2", fresh},
			{"cycle-3", stale},
		},
		nil,
	)
	res, err := Run(Options{
		ProjectRoot: repo,
		Now:         func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.EphemeralPruned != 2 {
		t.Errorf("EphemeralPruned = %d, want 2", res.EphemeralPruned)
	}
	// cycle-1 + cycle-3 should be gone; cycle-2 should remain.
	for _, name := range []string{"cycle-1", "cycle-3"} {
		path := filepath.Join(repo, ".evolve", "runs", name, ".ephemeral")
		if _, err := os.Stat(path); err == nil {
			t.Errorf("stale %s should have been pruned", path)
		}
	}
	freshPath := filepath.Join(repo, ".evolve", "runs", "cycle-2", ".ephemeral")
	if _, err := os.Stat(freshPath); err != nil {
		t.Errorf("fresh %s should have survived: %v", freshPath, err)
	}
}

// === Dry-run never removes anything ========================================
func TestRun_DryRun(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-10 * 24 * time.Hour)
	staleLog := now.Add(-45 * 24 * time.Hour) // exceed 30d log TTL
	repo := makeRepo(t,
		[]cycleSpec{{"cycle-1", stale}},
		[]logSpec{{"batch-old.log", staleLog}},
	)
	res, err := Run(Options{
		ProjectRoot: repo,
		DryRun:      true,
		Now:         func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.EphemeralPruned != 1 || res.LogFilesPruned != 1 {
		t.Errorf("dry-run counts = (eph=%d log=%d), want (1,1)",
			res.EphemeralPruned, res.LogFilesPruned)
	}
	// Both should still exist on disk.
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "runs", "cycle-1", ".ephemeral")); err != nil {
		t.Errorf("dry-run removed ephemeral dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "dispatch-logs", "batch-old.log")); err != nil {
		t.Errorf("dry-run removed log file: %v", err)
	}
	if len(res.EphemeralPaths) != 1 || len(res.LogPaths) != 1 {
		t.Errorf("dry-run paths not captured: %+v", res)
	}
}

// === Old batch logs get pruned ============================================
func TestRun_PrunesOldLogs(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-45 * 24 * time.Hour) // 45 days
	fresh := now.Add(-10 * 24 * time.Hour) // 10 days

	repo := makeRepo(t, nil, []logSpec{
		{"batch-old.log", stale},
		{"batch-recent.log", fresh},
		{"unrelated.txt", stale}, // wrong prefix; must NOT prune
	})
	res, err := Run(Options{
		ProjectRoot: repo,
		Now:         func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.LogFilesPruned != 1 {
		t.Errorf("LogFilesPruned = %d, want 1", res.LogFilesPruned)
	}
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "dispatch-logs", "batch-old.log")); err == nil {
		t.Error("batch-old.log should have been pruned")
	}
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "dispatch-logs", "batch-recent.log")); err != nil {
		t.Error("batch-recent.log should have survived")
	}
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "dispatch-logs", "unrelated.txt")); err != nil {
		t.Error("unrelated.txt should not have been touched")
	}
}

// === Quiet mode suppresses output unless something was pruned =============
func TestRun_QuietNoOp(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	repo := makeRepo(t, nil, nil) // nothing to prune
	var buf bytes.Buffer
	_, err := Run(Options{
		ProjectRoot: repo,
		Quiet:       true,
		Stderr:      &buf,
		Now:         func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("quiet mode emitted output on no-op: %q", buf.String())
	}
}

func TestRun_QuietWithActivity(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-10 * 24 * time.Hour)
	repo := makeRepo(t, []cycleSpec{{"cycle-1", stale}}, nil)
	var buf bytes.Buffer
	_, err := Run(Options{
		ProjectRoot: repo,
		Quiet:       true,
		Stderr:      &buf,
		Now:         func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(buf.String(), "pruned 1 ephemeral dirs") {
		t.Errorf("quiet mode missing activity summary: %q", buf.String())
	}
}

// === Custom TTLs honored ===================================================
func TestRun_CustomTTLs(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	twoDaysAgo := now.Add(-2 * 24 * time.Hour)
	repo := makeRepo(t, []cycleSpec{{"cycle-1", twoDaysAgo}}, nil)
	// TTL = 1 day — 2-day-old dir is stale.
	res, err := Run(Options{
		ProjectRoot: repo,
		TrackerTTL:  1 * 24 * time.Hour,
		Now:         func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.EphemeralPruned != 1 {
		t.Errorf("EphemeralPruned = %d, want 1 (TTL=1d, dir age=2d)", res.EphemeralPruned)
	}
}

// === Missing dirs are no-ops (don't error) =================================
func TestRun_MissingRunsDir(t *testing.T) {
	repo := t.TempDir() // no .evolve/runs/ at all
	res, err := Run(Options{
		ProjectRoot: repo,
		Now:         func() time.Time { return time.Now() },
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.EphemeralPruned != 0 || res.LogFilesPruned != 0 {
		t.Errorf("missing dirs should yield 0/0 prunes, got %+v", res)
	}
}

// === Empty ProjectRoot → error ============================================
func TestRun_EmptyProjectRoot(t *testing.T) {
	_, err := Run(Options{})
	if err == nil {
		t.Error("want error for empty ProjectRoot")
	}
	if !errors.Is(err, err) || !strings.Contains(err.Error(), "ProjectRoot") {
		t.Errorf("err = %v, want contains 'ProjectRoot'", err)
	}
}

// === Override dirs honored ================================================
func TestRun_OverrideDirs(t *testing.T) {
	d := t.TempDir()
	customRuns := filepath.Join(d, "custom-runs")
	customLogs := filepath.Join(d, "custom-logs")
	if err := os.MkdirAll(filepath.Join(customRuns, "cycle-x", ".ephemeral"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(filepath.Join(customRuns, "cycle-x", ".ephemeral"), stale, stale); err != nil {
		t.Fatalf("chtime: %v", err)
	}

	res, err := Run(Options{
		ProjectRoot:     d,
		RunsDir:         customRuns,
		DispatchLogsDir: customLogs,
		Now:             func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.EphemeralPruned != 1 {
		t.Errorf("EphemeralPruned = %d, want 1 (override dir)", res.EphemeralPruned)
	}
}

// === Fresh dirs survive (mtime within TTL) =================================
func TestRun_FreshEphemeralSurvives(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	repo := makeRepo(t, []cycleSpec{{"cycle-1", now.Add(-1 * time.Hour)}}, nil)
	res, err := Run(Options{
		ProjectRoot: repo,
		Now:         func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.EphemeralPruned != 0 {
		t.Errorf("EphemeralPruned = %d, want 0 (dir age = 1h < TTL)", res.EphemeralPruned)
	}
}
