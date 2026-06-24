//go:build integration

package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/runscope"
)

// TestGitWorktree_ConcurrentSiblingsNoBranchCollision reproduces the multi-stream
// failure: several `evolve loop` runs, each in its OWN worktree of the SAME repo,
// every one provisioning cycle 1. git worktree branch names — and, under a shared
// EVOLVE_WORKTREE_BASE, the directory path — are GLOBAL to one object store, so a
// bare `cycle-1` branch/dir from the first run made the second run's
// `git worktree add -B cycle-1` fail ("'cycle-1' is already used by worktree …");
// that run then fell back to the main tree and FAILED on the tree-diff guard.
// After the runscope fix each cycle name embeds the per-root lane, so sibling
// roots get distinct branches AND distinct dirs and both provision cleanly.
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

	// Force a SHARED worktree base for BOTH roots — the strongest form of the bug:
	// without the lane, both runs target the same dir (<shared>/cycle-1) AND the
	// same global branch (cycle-1), exercising both collision axes at once.
	sharedBase := t.TempDir()
	g := gitWorktree{baseOverride: sharedBase}
	wtMain, err := g.Create(mainRoot, 1)
	if err != nil {
		t.Fatalf("Create(mainRoot, 1): %v", err)
	}
	defer func() { _ = g.Cleanup(mainRoot, wtMain) }()

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
			t.Fatalf("symbolic-ref in %s (detached?): %v", wt, err)
		}
		return strings.TrimSpace(string(out))
	}
	bMain, bSib := branchOf(wtMain), branchOf(wtSibling)
	if bMain == bSib {
		t.Fatalf("sibling cycle branches collide: both %q", bMain)
	}
	if want := runscope.New(runscope.LaneFromRoot(mainRoot), "", 1).CycleBranch(); bMain != want {
		t.Errorf("main branch = %q, want %q", bMain, want)
	}
	if want := runscope.New(runscope.LaneFromRoot(sibling), "", 1).CycleBranch(); bSib != want {
		t.Errorf("sibling branch = %q, want %q", bSib, want)
	}
}
