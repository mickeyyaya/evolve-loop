//go:build integration

package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/runscope"
)

// TestGitWorktree_CreateUsesNamedBranch validates the production provisioner
// against real git: the worktree must be on a NAMED branch (cycle-<lane>-<N>),
// not a detached HEAD — worktree-aware ship resolves the branch via symbolic-ref
// and ff-merges it, so a detached worktree would break every cycle's ship. The
// branch embeds the runscope lane so concurrent sibling worktrees don't collide.
func TestGitWorktree_CreateUsesNamedBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-q", "-m", "init")

	g := gitWorktree{}
	wt, err := g.Create(root, 77)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer func() { _ = g.Cleanup(root, wt) }()

	out, err := exec.Command("git", "-C", wt, "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		t.Fatalf("worktree is detached (symbolic-ref failed): %v — ship would break", err)
	}
	wantBranch := runscope.New(runscope.LaneFromRoot(root), "", 77).CycleBranch()
	if got := strings.TrimSpace(string(out)); got != wantBranch {
		t.Fatalf("worktree branch = %q, want %q", got, wantBranch)
	}

	// linkGuardDeps must expose the dispatcher binary at the gitignored hook
	// path so the trust-kernel hooks resolve inside the worktree.
	if fi, err := os.Lstat(filepath.Join(wt, "go", "bin", "evolve")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("worktree go/bin/evolve should be a symlink (linkGuardDeps); lstat err=%v", err)
	}

	// Idempotent reuse returns the same valid worktree.
	if wt2, err := g.Create(root, 77); err != nil || wt2 != wt {
		t.Fatalf("reuse: got (%q, %v), want (%q, nil)", wt2, err, wt)
	}

	_ = g.Cleanup(root, wt)
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Fatalf("worktree not removed after Cleanup: stat err=%v", err)
	}
}

// branchExists lists local branches matching name and reports whether any
// matched — the real-git ground truth for "did Cleanup delete the branch".
func branchExists(t *testing.T, root, name string) bool {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "branch", "--list", name).Output()
	if err != nil {
		t.Fatalf("git branch --list %s: %v", name, err)
	}
	return strings.TrimSpace(string(out)) != ""
}

func initRealGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-q", "-m", "init")
	return root
}

// TestGitWorktree_Cleanup_DeletesMergedBranch (S3, workspace-hygiene plan):
// a cycle branch that never diverged from HEAD (the common ship-succeeded
// case — main was fast-forwarded onto it) is trivially merged, so Cleanup
// must delete it after removing the worktree — the fix for the 106
// never-deleted `cycle-*` branches the plan's audit found.
func TestGitWorktree_Cleanup_DeletesMergedBranch(t *testing.T) {
	root := initRealGitRepo(t)
	g := gitWorktree{}
	wt, err := g.Create(root, 41)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	branch := runscope.New(runscope.LaneFromRoot(root), "", 41).CycleBranch()
	if !branchExists(t, root, branch) {
		t.Fatalf("setup: branch %s should exist right after Create", branch)
	}

	if err := g.Cleanup(root, wt); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if branchExists(t, root, branch) {
		t.Errorf("branch %s still exists after Cleanup of a merged (unchanged) cycle branch — should have been deleted", branch)
	}
}

// TestGitWorktree_Cleanup_UnmergedBranchSurvives (S3): a branch carrying a
// commit that was never merged to the base (a ship-fail-and-abandon case)
// must survive Cleanup — git's own `branch -d` merge-check is the safety net
// against silently discarding evidence of unshipped work.
func TestGitWorktree_Cleanup_UnmergedBranchSurvives(t *testing.T) {
	root := initRealGitRepo(t)
	g := gitWorktree{}
	wt, err := g.Create(root, 42)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	branch := runscope.New(runscope.LaneFromRoot(root), "", 42).CycleBranch()

	// Diverge: commit inside the worktree, never merged back to root's HEAD.
	if err := os.WriteFile(filepath.Join(wt, "unshipped.txt"), []byte("wip"), 0o644); err != nil {
		t.Fatal(err)
	}
	commit := exec.Command("git", "-C", wt, "add", ".")
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd := exec.Command("git", "-C", wt, "commit", "-q", "-m", "unshipped work")
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	if err := g.Cleanup(root, wt); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if !branchExists(t, root, branch) {
		t.Errorf("branch %s was deleted despite carrying an unmerged commit — evidence of unshipped work lost", branch)
	}
}
