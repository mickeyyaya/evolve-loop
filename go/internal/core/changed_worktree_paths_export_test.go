package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// RED contract for cycle-1150 / wire-docsfloor-verify-cli.
//
// The CLI self-check (`evolve phase verify build`) needs the same changed-path
// derivation the host-side docs-floor reviewer already uses. That logic lives
// in the unexported changedWorktreePaths (phase_bindings.go): re-implementing
// git-diff derivation in internal/cli/phasecmd would put two answers to "what
// did this cycle change?" in the tree, and the gate and the self-check would
// drift (the ADR-0034 no-drift invariant). This test pins the single source
// with a projection: one EXPORTED wrapper, same semantics.

// changedPathsRepoFixture builds a git repo with one base commit, then applies
// a tracked modification and an untracked addition — the two shapes
// changedWorktreePaths must both report.
func changedPathsRepoFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "t@t.t")
	run("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "base.txt")
	run("commit", "-q", "-m", "base")

	// Tracked modification.
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Untracked addition under an architecture surface.
	newFile := filepath.Join(dir, "go", "internal", "policy", "policy.go")
	if err := os.MkdirAll(filepath.Dir(newFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newFile, []byte("package policy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestChangedWorktreePaths_ExportedForCLIConsumers — AC5. The exported wrapper
// must exist and return BOTH tracked changes vs HEAD and untracked additions,
// repo-relative, so the CLI can feed deliverable.VerifyBuildWithChangedPaths
// the same set the host reviewer sees. Compiling at all is half the contract:
// the name must be exported.
func TestChangedWorktreePaths_ExportedForCLIConsumers(t *testing.T) {
	dir := changedPathsRepoFixture(t)

	got := ChangedWorktreePaths(context.Background(), dir)

	seen := map[string]bool{}
	for _, p := range got {
		seen[p] = true
	}
	for _, want := range []string{"base.txt", "go/internal/policy/policy.go"} {
		if !seen[want] {
			t.Errorf("ChangedWorktreePaths = %v, missing %q (tracked diff + untracked adds must both surface)", got, want)
		}
	}
}

// TestChangedWorktreePaths_EmptyWorktreeIsEmpty — the fail-open edge. A clean
// repo (and a path that is not a repo at all) must yield no paths rather than a
// spurious entry: an empty change set is what makes the docs floor SKIP instead
// of manufacturing a violation.
func TestChangedWorktreePaths_EmptyWorktreeIsEmpty(t *testing.T) {
	t.Run("clean repo", func(t *testing.T) {
		dir := t.TempDir()
		for _, args := range [][]string{
			{"init", "-q"}, {"config", "user.email", "t@t.t"}, {"config", "user.name", "t"},
		} {
			cmd := exec.Command("git", args...)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"add", "base.txt"}, {"commit", "-q", "-m", "base"}} {
			cmd := exec.Command("git", args...)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
		if got := ChangedWorktreePaths(context.Background(), dir); len(got) != 0 {
			t.Errorf("clean repo: ChangedWorktreePaths = %v, want empty", got)
		}
	})

	t.Run("not a repo", func(t *testing.T) {
		if got := ChangedWorktreePaths(context.Background(), t.TempDir()); len(got) != 0 {
			t.Errorf("non-repo: ChangedWorktreePaths = %v, want empty (fail open, never a fabricated path)", got)
		}
	})
}
