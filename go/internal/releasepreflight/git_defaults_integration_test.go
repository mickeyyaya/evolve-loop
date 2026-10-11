//go:build integration

package releasepreflight

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestDefaultGitClean_ReportsCleanDirtyAndUnusableTrees(t *testing.T) {
	repo := gittest.Fixture(t)
	if err := os.WriteFile(filepath.Join(repo.Dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")

	clean, err := defaultGitClean(repo.Dir)
	if !clean || err != nil {
		t.Fatalf("committed tree: defaultGitClean = %v, %v; want true, nil", clean, err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "untracked.txt"), []byte("u\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if clean, err = defaultGitClean(repo.Dir); !clean || err != nil {
		t.Fatalf("untracked-only tree: defaultGitClean = %v, %v; want true, nil (diff HEAD ignores untracked)", clean, err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "a.txt"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if clean, err = defaultGitClean(repo.Dir); clean || err != nil {
		t.Fatalf("edited tree: defaultGitClean = %v, %v; want false, nil", clean, err)
	}
	unborn := gittest.Fixture(t)
	if clean, err = defaultGitClean(unborn.Dir); clean || err == nil || !strings.HasPrefix(err.Error(), "git diff failed: exit status ") {
		t.Fatalf("unborn HEAD: defaultGitClean = %v, %v; want false and the git diff failure", clean, err)
	}
}

func TestDefaultGitClean_MissingGitIsTheLaunchError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	clean, err := defaultGitClean(t.TempDir())

	if clean || !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("defaultGitClean = %v, %v; want false and exec.ErrNotFound", clean, err)
	}
}

func TestDefaultHeadSHA_ReturnsTheTrimmedFullHead(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Git("commit", "-q", "--allow-empty", "-m", "head")

	got, err := defaultHeadSHA(repo.Dir)

	if err != nil || got != repo.Git("rev-parse", "HEAD") || len(got) != 40 {
		t.Fatalf("defaultHeadSHA = %q, %v; want the 40-hex HEAD without a newline", got, err)
	}
}

func TestDefaultHeadSHA_UnbornHeadIsAnError(t *testing.T) {
	repo := gittest.Fixture(t)

	got, err := defaultHeadSHA(repo.Dir)

	var exitErr *exec.ExitError
	if got != "" || !errors.As(err, &exitErr) {
		t.Fatalf("defaultHeadSHA = %q, %v; want empty and the rev-parse exit", got, err)
	}
}
