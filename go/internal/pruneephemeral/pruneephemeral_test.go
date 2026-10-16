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

func makeRepo(t *testing.T, cycles []cycleSpec, logs []logSpec) string {
	t.Helper()
	d := t.TempDir()
	runsDir := filepath.Join(d, ".evolve", "runs")
	for _, c := range cycles {
		dir := filepath.Join(runsDir, c.name, ".ephemeral")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
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

func TestRun_PrunesOldEphemerals(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-10 * 24 * time.Hour)
	fresh := now.Add(-2 * 24 * time.Hour)

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

func TestRun_DryRun(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-10 * 24 * time.Hour)
	staleLog := now.Add(-45 * 24 * time.Hour)
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

func TestRun_PrunesOldLogs(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-45 * 24 * time.Hour)
	fresh := now.Add(-10 * 24 * time.Hour)

	repo := makeRepo(t, nil, []logSpec{
		{"batch-old.log", stale},
		{"batch-recent.log", fresh},
		{"unrelated.txt", stale},
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

func TestRun_QuietNoOp(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	repo := makeRepo(t, nil, nil)
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

func TestRun_CustomTTLs(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	twoDaysAgo := now.Add(-2 * 24 * time.Hour)
	repo := makeRepo(t, []cycleSpec{{"cycle-1", twoDaysAgo}}, nil)
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

func TestRun_MissingRunsDir(t *testing.T) {
	repo := t.TempDir()
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

func TestRun_EmptyProjectRoot(t *testing.T) {
	_, err := Run(Options{})
	if err == nil {
		t.Error("want error for empty ProjectRoot")
	}
	if !errors.Is(err, err) || !strings.Contains(err.Error(), "ProjectRoot") {
		t.Errorf("err = %v, want contains 'ProjectRoot'", err)
	}
}

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

func TestRun_NonDirEntryInRunsSkipped(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-10 * 24 * time.Hour)
	repo := makeRepo(t, []cycleSpec{{"cycle-1", stale}}, nil)
	stray := filepath.Join(repo, ".evolve", "runs", "stray.txt")
	if err := os.WriteFile(stray, []byte("x"), 0o644); err != nil {
		t.Fatalf("write stray: %v", err)
	}
	if err := os.Chtimes(stray, stale, stale); err != nil {
		t.Fatalf("chtime stray: %v", err)
	}
	res, err := Run(Options{ProjectRoot: repo, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.EphemeralPruned != 1 {
		t.Errorf("EphemeralPruned = %d, want 1 (stray file ignored)", res.EphemeralPruned)
	}
	if _, err := os.Stat(stray); err != nil {
		t.Errorf("stray file should be untouched: %v", err)
	}
}

func TestRun_CycleWithoutEphemeralSkipped(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-10 * 24 * time.Hour)
	repo := makeRepo(t, []cycleSpec{{"cycle-1", stale}}, nil)
	bare := filepath.Join(repo, ".evolve", "runs", "cycle-2")
	if err := os.MkdirAll(bare, 0o755); err != nil {
		t.Fatalf("mkdir bare: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bare, "report.md"), []byte("kept"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	res, err := Run(Options{ProjectRoot: repo, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.EphemeralPruned != 1 {
		t.Errorf("EphemeralPruned = %d, want 1 (cycle-2 has no .ephemeral)", res.EphemeralPruned)
	}
}

func TestRun_DispatchLogSubdirSkipped(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-45 * 24 * time.Hour)
	repo := makeRepo(t, nil, []logSpec{{"batch-old.log", stale}})
	subdir := filepath.Join(repo, ".evolve", "dispatch-logs", "batch-dir.log")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	if err := os.Chtimes(subdir, stale, stale); err != nil {
		t.Fatalf("chtime subdir: %v", err)
	}
	res, err := Run(Options{ProjectRoot: repo, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.LogFilesPruned != 1 {
		t.Errorf("LogFilesPruned = %d, want 1 (subdir ignored)", res.LogFilesPruned)
	}
	if _, err := os.Stat(subdir); err != nil {
		t.Errorf("batch-dir.log subdir should be untouched: %v", err)
	}
}

func TestRun_EphemeralRemovalFailureNotCounted(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory write permissions")
	}
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-10 * 24 * time.Hour)
	repo := makeRepo(t, []cycleSpec{{"cycle-1", stale}}, nil)
	cycleDir := filepath.Join(repo, ".evolve", "runs", "cycle-1")
	if err := os.Chmod(cycleDir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(cycleDir, 0o755) })

	var buf bytes.Buffer
	res, err := Run(Options{ProjectRoot: repo, Stderr: &buf, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.EphemeralPruned != 0 {
		t.Errorf("EphemeralPruned = %d, want 0 (removal failed)", res.EphemeralPruned)
	}
	if !strings.Contains(buf.String(), "WARN: failed to remove") {
		t.Errorf("expected WARN log on removal failure, got: %q", buf.String())
	}
}

func TestRun_LogRemovalFailureNotCounted(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory write permissions")
	}
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-45 * 24 * time.Hour)
	repo := makeRepo(t, nil, []logSpec{{"batch-old.log", stale}})
	logsDir := filepath.Join(repo, ".evolve", "dispatch-logs")
	if err := os.Chmod(logsDir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(logsDir, 0o755) })

	var buf bytes.Buffer
	res, err := Run(Options{ProjectRoot: repo, Stderr: &buf, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.LogFilesPruned != 0 {
		t.Errorf("LogFilesPruned = %d, want 0 (removal failed)", res.LogFilesPruned)
	}
	if !strings.Contains(buf.String(), "WARN: failed to remove") {
		t.Errorf("expected WARN log on removal failure, got: %q", buf.String())
	}
}

func TestRun_DanglingLogSymlinkSkipped(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-45 * 24 * time.Hour)
	repo := makeRepo(t, nil, []logSpec{{"batch-old.log", stale}})
	link := filepath.Join(repo, ".evolve", "dispatch-logs", "batch-dangling.log")
	if err := os.Symlink(filepath.Join(repo, "does-not-exist"), link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	res, err := Run(Options{ProjectRoot: repo, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.LogFilesPruned != 1 {
		t.Errorf("LogFilesPruned = %d, want 1 (dangling symlink skipped)", res.LogFilesPruned)
	}
}

func TestRun_NilNowUsesWallClock(t *testing.T) {
	stale := time.Now().Add(-100 * 24 * time.Hour)
	repo := makeRepo(t, []cycleSpec{{"cycle-1", stale}}, nil)
	res, err := Run(Options{ProjectRoot: repo})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.EphemeralPruned != 1 {
		t.Errorf("EphemeralPruned = %d, want 1 (default wall clock should treat 100d-old as stale)", res.EphemeralPruned)
	}
}

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
