//go:build integration

package resolvellm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func chdirForTest(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(prev); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	})
}

func TestGitRoot_IsTheToplevelOfTheCurrentRepository(t *testing.T) {
	repo := gittest.Fixture(t)
	sub := filepath.Join(repo.Dir, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	chdirForTest(t, sub)
	want, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}

	if got := gitRoot(); got != want {
		t.Fatalf("gitRoot() = %q, want the toplevel %q without a newline", got, want)
	}
}

func TestGitRoot_OutsideARepositoryIsEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	chdirForTest(t, dir)

	if got := gitRoot(); got != "" {
		t.Fatalf("gitRoot() = %q, want empty outside a repository", got)
	}
}
