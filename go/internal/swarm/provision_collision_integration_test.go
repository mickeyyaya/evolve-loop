//go:build integration

package swarm

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGitWorkerProvisioner_ConcurrentSiblingsNoCollision is the swarm mirror of
// the core sibling-collision test: two worktrees of one repo each provision a
// cycle-1 integration + worker branch under a SHARED base. Pre-runscope both
// minted bare cycle-1-integration / cycle-1-w0 and collided on the global branch
// namespace; with the per-root lane each sibling gets distinct names.
func TestGitWorkerProvisioner_ConcurrentSiblingsNoCollision(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
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
	sibling := filepath.Join(t.TempDir(), "sibling")
	git(mainRoot, "worktree", "add", "-q", "-b", "stream-sibling", sibling, "HEAD")

	sharedBase := t.TempDir()
	ctx := context.Background()
	p := NewGitWorkerProvisioner(nil, sharedBase)

	iMain, err := p.CreateIntegration(ctx, mainRoot, 1)
	if err != nil {
		t.Fatalf("CreateIntegration(mainRoot): %v", err)
	}
	defer func() { _ = p.Cleanup(ctx, mainRoot, iMain) }()
	iSib, err := p.CreateIntegration(ctx, sibling, 1)
	if err != nil {
		t.Fatalf("CreateIntegration(sibling) collided: %v", err)
	}
	defer func() { _ = p.Cleanup(ctx, sibling, iSib) }()

	if iMain == iSib {
		t.Fatalf("sibling integration worktrees collide: both %q", iMain)
	}
	if !strings.Contains(filepath.Base(iMain), integBranchFor(mainRoot, 1)) {
		t.Errorf("main integration dir %q lacks lane-scoped name %q", iMain, integBranchFor(mainRoot, 1))
	}

	wMain, err := p.CreateWorker(ctx, mainRoot, 1, "w0", "")
	if err != nil {
		t.Fatalf("CreateWorker(mainRoot): %v", err)
	}
	defer func() { _ = p.Cleanup(ctx, mainRoot, wMain) }()
	wSib, err := p.CreateWorker(ctx, sibling, 1, "w0", "")
	if err != nil {
		t.Fatalf("CreateWorker(sibling) collided: %v", err)
	}
	defer func() { _ = p.Cleanup(ctx, sibling, wSib) }()
	if wMain == wSib {
		t.Fatalf("sibling worker worktrees collide: both %q", wMain)
	}
}
