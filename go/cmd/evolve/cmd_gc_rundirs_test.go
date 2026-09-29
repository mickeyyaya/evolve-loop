package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func gcAgedRunDirs(t *testing.T, projectRoot string, n int) []string {
	t.Helper()
	var dirs []string
	for i := 1; i <= n; i++ {
		dir := filepath.Join(projectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", i))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "run.json"), []byte(`{}`), 0o644); err != nil {
			t.Fatal(err)
		}
		mtime := time.Now().Add(-time.Duration(20+i) * 24 * time.Hour)
		if err := os.Chtimes(dir, mtime, mtime); err != nil {
			t.Fatal(err)
		}
		dirs = append(dirs, dir)
	}
	return dirs
}

func TestRunGC_AppliesTheRunDirRetentionLadder(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	gcFakeGoCache(t)
	dirs := gcAgedRunDirs(t, projectRoot, 12)
	gcWritePolicy(t, projectRoot, `{"runs":{"delete_after_days":14}}`)
	oldest := dirs[10:]

	var stdout, stderr bytes.Buffer
	runGC([]string{"--dry-run", "--project-root", projectRoot}, nil, &stdout, &stderr)
	for _, d := range oldest {
		if !strings.Contains(stdout.String(), "WOULD-DELETE "+d) {
			t.Errorf("--dry-run does not preview deleting %s:\n%s\nstderr=%s", filepath.Base(d), stdout.String(), stderr.String())
		}
		if _, err := os.Stat(d); err != nil {
			t.Fatalf("--dry-run deleted %s", filepath.Base(d))
		}
	}

	stdout.Reset()
	runGC([]string{"--project-root", projectRoot}, nil, &stdout, &stderr)
	for i, d := range dirs {
		_, err := os.Stat(d)
		if gone := os.IsNotExist(err); gone != (i >= 10) {
			t.Errorf("%s gone=%v, want %v: keep_full keeps the newest 10, delete_after_days takes the rest\n%s", filepath.Base(d), gone, i >= 10, stdout.String())
		}
	}
}

func TestRunGC_ARunDirRetentionCannotDeleteFailsTheRun(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	gcFakeGoCache(t)
	gcAgedRunDirs(t, projectRoot, 12)
	gcWritePolicy(t, projectRoot, `{"runs":{"delete_after_days":14}}`)
	runs := filepath.Join(projectRoot, ".evolve", "runs")
	if err := os.Chmod(runs, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(runs, 0o755) })

	var stdout, stderr bytes.Buffer
	rc := runGC([]string{"--project-root", projectRoot}, nil, &stdout, &stderr)

	if rc != 1 || !strings.Contains(stderr.String(), "run-dir retention partial") {
		t.Errorf("rc=%d stderr=%q: a deletion the retention step could not perform must fail the run", rc, stderr.String())
	}
}
