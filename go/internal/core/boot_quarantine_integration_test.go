//go:build integration

package core

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func seededQuarantineRepo(t *testing.T, files map[string]string) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	for rel, body := range files {
		path := filepath.Join(repo.Dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFileT(t, path, body)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "seed")
	return repo
}

func TestPorcelainPaths_ReportsTheDestinationOfEveryKindOfDirt(t *testing.T) {
	repo := seededQuarantineRepo(t, map[string]string{"old.go": "package a\n", "edit.go": "package a\n", "gone.go": "package a\n"})
	repo.Git("mv", "old.go", "renamed.go")
	writeFileT(t, filepath.Join(repo.Dir, "edit.go"), "package a\n\nvar x = 1\n")
	if err := os.Remove(filepath.Join(repo.Dir, "gone.go")); err != nil {
		t.Fatal(err)
	}
	writeFileT(t, filepath.Join(repo.Dir, "with space.go"), "package a\n")
	writeFileT(t, filepath.Join(repo.Dir, "z"), "one-character name\n")
	if err := os.MkdirAll(filepath.Join(repo.Dir, "newdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFileT(t, filepath.Join(repo.Dir, "newdir", "n.go"), "package a\n")

	got, err := porcelainPaths(context.Background(), repo.Dir)

	if err != nil {
		t.Fatalf("porcelainPaths: %v", err)
	}
	slices.Sort(got)
	want := []string{"edit.go", "gone.go", "newdir/", "renamed.go", "with space.go", "z"}
	if !slices.Equal(got, want) {
		t.Fatalf("porcelainPaths = %q, want %q", got, want)
	}
}

func TestPorcelainPaths_CleanTreeHasNoPaths(t *testing.T) {
	repo := seededQuarantineRepo(t, map[string]string{"a.go": "package a\n"})

	got, err := porcelainPaths(context.Background(), repo.Dir)

	if err != nil || got != nil {
		t.Fatalf("porcelainPaths = %q, %v; want nil, nil", got, err)
	}
}

func TestPorcelainPaths_NotARepositoryIsAWrappedStatusError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))

	got, err := porcelainPaths(context.Background(), dir)

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || !strings.HasPrefix(err.Error(), "quarantine: git status: ") || got != nil {
		t.Fatalf("porcelainPaths = %q, %v; want nil and the wrapped git status exit", got, err)
	}
}

func TestQuarantineDirtyTree_StashesOnlyLeakedSourceUnderTheLabel(t *testing.T) {
	repo := seededQuarantineRepo(t, map[string]string{
		"go/internal/x/x.go":  "package x\n",
		".evolve/state.json":  "{}\n",
		"knowledge-base/k.md": "k\n",
	})
	writeFileT(t, filepath.Join(repo.Dir, "go/internal/x/x.go"), "package x\n\nvar leaked = 1\n")
	writeFileT(t, filepath.Join(repo.Dir, ".evolve/state.json"), "{\"cycle\":9}\n")
	writeFileT(t, filepath.Join(repo.Dir, "knowledge-base/k.md"), "k2\n")
	if err := os.MkdirAll(filepath.Join(repo.Dir, "go/bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFileT(t, filepath.Join(repo.Dir, "go/bin/evolve"), "binary")
	writeFileT(t, filepath.Join(repo.Dir, "go/internal/x/new.go"), "package x\n")

	stashed, err := QuarantineDirtyTree(context.Background(), repo.Dir, "boot-quarantine-cycle-42")

	if err != nil || !stashed {
		t.Fatalf("QuarantineDirtyTree = %v, %v; want true, nil", stashed, err)
	}
	if got := repo.Git("stash", "list", "--format=%s"); !strings.HasSuffix(got, ": boot-quarantine-cycle-42") || strings.Contains(got, "\n") {
		t.Fatalf("stash list = %q, want one stash named by the label", got)
	}
	stashedFiles := repo.Git("show", "--name-only", "--format=", "stash@{0}") + "\n" + repo.Git("show", "--name-only", "--format=", "stash@{0}^3")
	for _, want := range []string{"go/internal/x/x.go", "go/internal/x/new.go"} {
		if !strings.Contains(stashedFiles, want) {
			t.Fatalf("stash holds %q, want %s in it", stashedFiles, want)
		}
	}
	if strings.Contains(stashedFiles, "go/bin/evolve") || strings.Contains(stashedFiles, ".evolve/") || strings.Contains(stashedFiles, "knowledge-base/") {
		t.Fatalf("stash holds loop-managed paths: %q", stashedFiles)
	}
	if got := readFileT(t, filepath.Join(repo.Dir, ".evolve/state.json")); got != "{\"cycle\":9}\n" {
		t.Fatalf(".evolve dirt changed to %q, want it untouched", got)
	}
	if got := readFileT(t, filepath.Join(repo.Dir, "go/bin/evolve")); got != "binary" {
		t.Fatalf("ship binary changed to %q, want it untouched", got)
	}
}

func TestQuarantineDirtyTree_LoopManagedDirtAloneIsNoQuarantine(t *testing.T) {
	repo := seededQuarantineRepo(t, map[string]string{".evolve/state.json": "{}\n"})
	writeFileT(t, filepath.Join(repo.Dir, ".evolve/state.json"), "{\"cycle\":1}\n")

	stashed, err := QuarantineDirtyTree(context.Background(), repo.Dir, "label")

	if err != nil || stashed {
		t.Fatalf("QuarantineDirtyTree = %v, %v; want false, nil", stashed, err)
	}
	if got := repo.Git("stash", "list"); got != "" {
		t.Fatalf("stash list = %q, want no stash", got)
	}
}

func TestQuarantineDirtyTree_StatusFailureIsReturned(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))

	stashed, err := QuarantineDirtyTree(context.Background(), dir, "label")

	if stashed || err == nil || !strings.HasPrefix(err.Error(), "quarantine: git status: ") {
		t.Fatalf("QuarantineDirtyTree = %v, %v; want false and the status error", stashed, err)
	}
}

func TestQuarantineDirtyTree_StashFailureCarriesGitOutput(t *testing.T) {
	repo := gittest.Fixture(t)
	writeFileT(t, filepath.Join(repo.Dir, "leak.go"), "package a\n")

	stashed, err := QuarantineDirtyTree(context.Background(), repo.Dir, "label")

	if stashed || err == nil || !strings.HasPrefix(err.Error(), "quarantine: git stash: ") || !strings.Contains(err.Error(), "initial commit") {
		t.Fatalf("QuarantineDirtyTree = %v, %v; want false and the stash error with git's reason", stashed, err)
	}
	if got := readFileT(t, filepath.Join(repo.Dir, "leak.go")); got != "package a\n" {
		t.Fatalf("leaked file changed to %q after a failed stash", got)
	}
}

func TestQuarantineDirtyTree_CanceledContextNeverStashes(t *testing.T) {
	repo := seededQuarantineRepo(t, map[string]string{"a.go": "package a\n"})
	writeFileT(t, filepath.Join(repo.Dir, "a.go"), "package a\n\nvar b = 1\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	stashed, err := QuarantineDirtyTree(ctx, repo.Dir, "label")

	if stashed || !errors.Is(err, context.Canceled) {
		t.Fatalf("QuarantineDirtyTree = %v, %v; want false and context.Canceled", stashed, err)
	}
	if got := readFileT(t, filepath.Join(repo.Dir, "a.go")); got != "package a\n\nvar b = 1\n" {
		t.Fatalf("dirt changed to %q under a canceled context", got)
	}
}
