package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestProductionFloorCheck_RunsTheHistoryFloor(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	page := filepath.Join(repo.Dir, "x.go")
	if err := os.WriteFile(page, []byte("package x\n\n// F is the entry point.\n// It used to live in cycle 42's retry loop.\nfunc F() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	if err := os.WriteFile(page, []byte("package x\n\nfunc F() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"comment_floor":{"history_stage":"enforce"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got := core.BuildHandoffFloor{productionFloorCheck()}.Failures(context.Background(), core.ReviewInput{Phase: string(core.PhaseBuild), ProjectRoot: root, Worktree: repo.Dir, WorktreeBaseSHA: base})

	if !strings.Contains(strings.Join(got, "\n"), "x.go:3") {
		t.Fatalf("the production floor does not run the history floor: %v", got)
	}
}
