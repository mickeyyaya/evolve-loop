package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// worktree_normalize_test.go — Option C for the cycle-156 incident
// (docs/incidents/cycle-156-builder-commit-vs-audit-pending-diff.md).
//
// A builder is instructed to `git commit ... [worktree-build]` (evolve-builder.md:235),
// but the auditor + binding inspect `git diff HEAD`, which is EMPTY after a commit.
// normalizeWorktreeToBase soft-resets the builder's commits back to the cycle base so
// the work becomes PENDING changes again — the state both the auditor and the binding
// assume. These tests pin that contract.

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

// TestNormalizeWorktreeToBase_UncommitsBuilderCommit is the RED-phase contract:
// a builder commit on top of base must be soft-reset to base so the file shows
// in `git diff HEAD`.
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
	// The feature file must now be a PENDING change visible to `git diff HEAD`.
	diff := gitInRepo(t, dir, "diff", "HEAD", "--name-only")
	if !strings.Contains(diff, "feature.go") {
		t.Fatalf("feature.go must be pending after normalize; git diff HEAD --name-only=%q", diff)
	}
}

// TestNormalizeWorktreeToBase_NoopWhenUncommitted: when the builder left changes
// UNCOMMITTED (HEAD already == base — the historical Claude-builder path), the
// helper is a no-op and the pending changes survive untouched.
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
