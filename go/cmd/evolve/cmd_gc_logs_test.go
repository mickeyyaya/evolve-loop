package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

func TestRunGC_DryRunWarnsWhenTheCurrentLoopLogStaysOverItsCap(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	gcFakeGoCache(t)
	gcWritePolicy(t, projectRoot, `{"logs":{"loop":{"max_total_mb":1}}}`)
	logs := filepath.Join(projectRoot, ".evolve", gcpolicy.LogsDir)
	run := filepath.Join(logs, "20261008T142501Z")
	if err := os.MkdirAll(run, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(run, gcpolicy.LoopLogName), bytes.Repeat([]byte("x"), 2<<20), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(run), filepath.Join(logs, gcpolicy.LogsCurrentLink)); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	runGC([]string{"--dry-run", "--project-root", projectRoot}, nil, &stdout, &stderr)

	if !strings.Contains(stderr.String(), "evolve gc: WARN: logs.loop.max_total_mb:") {
		t.Errorf("stderr does not warn that the current loop log stays over its cap:\n%s", stderr.String())
	}
	if _, err := os.Stat(run); err != nil {
		t.Errorf("the current run dir is gone after a dry run: %v", err)
	}
}

func TestRunGCHook_WarnsWhenTheCurrentLoopLogStaysOverItsCap(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	gcWritePolicy(t, root, `{"mode":"shadow","logs":{"loop":{"max_total_mb":1}}}`)
	run := filepath.Join(evolveDir, gcpolicy.LogsDir, "20261008T142501Z")
	if err := os.MkdirAll(run, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(run, gcpolicy.LoopLogName), bytes.Repeat([]byte("x"), 2<<20), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(run), filepath.Join(evolveDir, gcpolicy.LogsDir, gcpolicy.LogsCurrentLink)); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer

	runGCHook(loopConfig{EvolveDir: evolveDir}, t.TempDir(), &stderr)

	if !strings.Contains(stderr.String(), "[gc] WARN: logs.loop.max_total_mb:") {
		t.Errorf("the batch hook does not warn that the current loop log stays over its cap:\n%s", stderr.String())
	}
}
