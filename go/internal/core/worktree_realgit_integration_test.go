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
