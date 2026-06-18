//go:build integration

package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

// TestGitWorktree_ConcurrentSiblingsNoBranchCollision reproduces the multi-stream
// failure: several `evolve loop` runs, each in its OWN worktree of the SAME repo,
// every one provisioning cycle 1. git worktree branch names are GLOBAL to one
// object store, so a plain `cycle-1` branch from the first run made the second
// run's `git worktree add -B cycle-1` fail ("'cycle-1' is already used by
// worktree ..."); that run then fell back to the main tree and the cycle FAILED
// on the tree-diff guard. After the fix each cycle branch embeds
// gitexec.WorktreeToken(root), so sibling roots get distinct branches and both
// provision cleanly — the precondition for concurrent work streams.
func TestGitWorktree_ConcurrentSiblingsNoBranchCollision(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git -C %s %v: %v\n%s", dir, args, err, out)
		}
	}

	mainRoot := t.TempDir()
	git(mainRoot, "init", "-q")
	if err := os.WriteFile(filepath.Join(mainRoot, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(mainRoot, "add", ".")
	git(mainRoot, "commit", "-q", "-m", "init")

	// A SECOND worktree of the same repo — the sibling work stream.
	sibling := filepath.Join(t.TempDir(), "sibling")
	git(mainRoot, "worktree", "add", "-q", "-b", "stream-sibling", sibling, "HEAD")

	// Force a SHARED worktree base for BOTH roots. This is the strongest form of
	// the bug: without the token, both runs would target the same directory path
	// (<shared>/cycle-1) AND the same global branch (cycle-1) — exercising both
	// the directory-collision and the branch-collision axes at once.
	sharedBase := t.TempDir()
	t.Setenv("EVOLVE_WORKTREE_BASE", sharedBase)

	g := gitWorktree{}
	wtMain, err := g.Create(mainRoot, 1)
	if err != nil {
		t.Fatalf("Create(mainRoot, 1): %v", err)
	}
	defer func() { _ = g.Cleanup(mainRoot, wtMain) }()

	// Pre-fix this FAILED with "'cycle-1' is already used by worktree" (or a path
	// collision under the shared base).
	wtSibling, err := g.Create(sibling, 1)
	if err != nil {
		t.Fatalf("Create(sibling, 1) collided (the multi-stream bug): %v", err)
	}
	defer func() { _ = g.Cleanup(sibling, wtSibling) }()

	// Distinct directory paths even under one shared base (the dir-collision axis).
	if wtMain == wtSibling {
		t.Fatalf("sibling worktree dirs collide under a shared base: both %q", wtMain)
	}

	branchOf := func(wt string) string {
		out, err := exec.Command("git", "-C", wt, "symbolic-ref", "--short", "HEAD").Output()
		if err != nil {
			t.Fatalf("symbolic-ref in %s (worktree detached?): %v", wt, err)
		}
		return strings.TrimSpace(string(out))
	}
	bMain, bSib := branchOf(wtMain), branchOf(wtSibling)
	if bMain == bSib {
		t.Fatalf("sibling cycle branches collide: both %q", bMain)
	}
	if want := "cycle-" + gitexec.WorktreeToken(mainRoot) + "-1"; bMain != want {
		t.Errorf("main branch = %q, want %q", bMain, want)
	}
	if want := "cycle-" + gitexec.WorktreeToken(sibling) + "-1"; bSib != want {
		t.Errorf("sibling branch = %q, want %q", bSib, want)
	}
}
