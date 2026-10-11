//go:build integration

package repostate

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestTrackedTree_ListsOnlyTrackedFilesUnderTheDir(t *testing.T) {
	repo := gittest.Fixture(t)
	for rel, body := range map[string]string{"go/a.go": "a", "go/sub/b go.go": "b", "go-other/c.go": "c", "top.txt": "t"} {
		path := filepath.Join(repo.Dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "seed")
	if err := os.WriteFile(filepath.Join(repo.Dir, "go", "untracked.go"), []byte("u"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := TrackedTree(repo.Dir, "go")

	if want := []string{"go/a.go", "go/sub/b go.go"}; err != nil || !slices.Equal(got, want) {
		t.Fatalf("TrackedTree = %q, %v; want %q", got, err, want)
	}
	if empty, err := TrackedTree(repo.Dir, "absent"); err != nil || empty != nil {
		t.Fatalf("TrackedTree(absent) = %q, %v; want nil, nil", empty, err)
	}
}

func TestTrackedTree_NotARepositoryCarriesGitsReason(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))

	got, err := TrackedTree(dir, "go")

	var exitErr *exec.ExitError
	if got != nil || !errors.As(err, &exitErr) || !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("TrackedTree = %q, %v; want git's not-a-repository reason", got, err)
	}
}

func TestTrackedTree_MissingGitIsWrappedWithoutStderr(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	got, err := TrackedTree("/repo", "go")

	if got != nil || !errors.Is(err, exec.ErrNotFound) || !strings.HasPrefix(err.Error(), "git ls-files go in /repo: ") {
		t.Fatalf("TrackedTree = %q, %v; want the wrapped launch error", got, err)
	}
}

func TestTrackedTree_SilentGitFailureIsWrappedWithoutStderr(t *testing.T) {
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "git"), "#!/bin/sh\nexit 5\n")
	t.Setenv("PATH", bin)

	_, err := TrackedTree("/repo", "go")

	if err == nil || err.Error() != "git ls-files go in /repo: exit status 5" {
		t.Fatalf("err = %v, want the bare exit status", err)
	}
}
