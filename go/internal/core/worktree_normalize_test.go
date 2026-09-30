//go:build integration

package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitInRepo(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// newRepoWithBaseCommit creates a temp git repo with one base commit and returns
// (repoDir, baseSHA).
func newRepoWithBaseCommit(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	gitInRepo(t, dir, "init", "-q")
	gitInRepo(t, dir, "config", "user.email", "t@t.t")
	gitInRepo(t, dir, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInRepo(t, dir, "add", "-A")
	gitInRepo(t, dir, "commit", "-q", "-m", "base")
	return dir, gitInRepo(t, dir, "rev-parse", "HEAD")
}

func TestNormalizeWorktreeToBase_UncommitsBuilderCommit(t *testing.T) {
	t.Parallel()
	dir, base := newRepoWithBaseCommit(t)
	// Simulate the builder: write a feature file + commit it [worktree-build].
	if err := os.WriteFile(filepath.Join(dir, "feature.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInRepo(t, dir, "add", "-A")
	gitInRepo(t, dir, "commit", "-q", "-m", "feat: feature [worktree-build]")
	if head := gitInRepo(t, dir, "rev-parse", "HEAD"); head == base {
		t.Fatal("precondition: HEAD should be ahead of base after the builder commit")
	}

	normalizeWorktreeToBase(context.Background(), dir, base)

	if head := gitInRepo(t, dir, "rev-parse", "HEAD"); head != base {
		t.Fatalf("HEAD=%s, want base=%s (soft-reset should move HEAD to base)", head, base)
	}
	diff := gitInRepo(t, dir, "diff", "HEAD", "--name-only")
	if !strings.Contains(diff, "feature.go") {
		t.Fatalf("feature.go must be pending after normalize; git diff HEAD --name-only=%q", diff)
	}
}

func TestNormalizeWorktreeToBase_NoopWhenUncommitted(t *testing.T) {
	t.Parallel()
	dir, base := newRepoWithBaseCommit(t)
	if err := os.WriteFile(filepath.Join(dir, "pending.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInRepo(t, dir, "add", "-A") // staged but NOT committed

	normalizeWorktreeToBase(context.Background(), dir, base)

	if head := gitInRepo(t, dir, "rev-parse", "HEAD"); head != base {
		t.Fatalf("HEAD must remain at base on no-op; got %s want %s", head, base)
	}
	diff := gitInRepo(t, dir, "diff", "HEAD", "--name-only")
	if !strings.Contains(diff, "pending.go") {
		t.Fatalf("pending.go must survive the no-op; git diff HEAD --name-only=%q", diff)
	}
}
