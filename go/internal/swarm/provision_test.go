package swarm

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitInit makes a throwaway repo with one commit so `git worktree add` works.
func gitInit(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "t@t.local")
	run("config", "user.name", "T")
	run("config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "init")
	return root
}

func branchOf(t *testing.T, wt string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", wt, "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		t.Fatalf("symbolic-ref: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func TestGitWorkerProvisioner_IntegrationAndWorkers(t *testing.T) {
	root := gitInit(t)
	t.Setenv("EVOLVE_WORKTREE_BASE", filepath.Join(root, ".evolve", "worktrees"))
	ctx := context.Background()
	var linked []string
	p := NewGitWorkerProvisioner(func(wt, _ string) { linked = append(linked, wt) })

	integ, err := p.CreateIntegration(ctx, root, 5)
	if err != nil {
		t.Fatal(err)
	}
	if branchOf(t, integ) != "cycle-5-integration" {
		t.Errorf("integration branch = %q", branchOf(t, integ))
	}

	// Workers branch off the integration branch with a NAMED branch (symbolic-ref
	// resolvable, required by merge-train/ship).
	w0, err := p.CreateWorker(ctx, root, 5, "w0", "cycle-5-integration")
	if err != nil {
		t.Fatal(err)
	}
	if branchOf(t, w0) != "cycle-5-w0" {
		t.Errorf("worker branch = %q", branchOf(t, w0))
	}
	w1, err := p.CreateWorker(ctx, root, 5, "w1", "cycle-5-integration")
	if err != nil {
		t.Fatal(err)
	}
	if w0 == w1 {
		t.Error("workers must get distinct worktrees")
	}
	// linkGuardDeps ran for each provisioned worktree.
	if len(linked) != 3 {
		t.Errorf("linkGuardDeps should run per worktree (3), got %d", len(linked))
	}
}

func TestGitWorkerProvisioner_CreateWorkerIdempotent(t *testing.T) {
	root := gitInit(t)
	t.Setenv("EVOLVE_WORKTREE_BASE", filepath.Join(root, ".evolve", "worktrees"))
	ctx := context.Background()
	p := NewGitWorkerProvisioner(nil)
	_, _ = p.CreateIntegration(ctx, root, 1)
	a, err := p.CreateWorker(ctx, root, 1, "w0", "cycle-1-integration")
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.CreateWorker(ctx, root, 1, "w0", "cycle-1-integration") // reuse
	if err != nil {
		t.Fatalf("idempotent re-create failed: %v", err)
	}
	if a != b {
		t.Errorf("idempotent create should return same path: %q vs %q", a, b)
	}
}

func TestGitWorkerProvisioner_Cleanup(t *testing.T) {
	root := gitInit(t)
	t.Setenv("EVOLVE_WORKTREE_BASE", filepath.Join(root, ".evolve", "worktrees"))
	ctx := context.Background()
	p := NewGitWorkerProvisioner(nil)
	_, _ = p.CreateIntegration(ctx, root, 1)
	w0, err := p.CreateWorker(ctx, root, 1, "w0", "cycle-1-integration")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Cleanup(ctx, root, w0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(w0); !os.IsNotExist(err) {
		t.Errorf("worktree should be gone after cleanup, stat err=%v", err)
	}
	// Cleanup of empty path is a no-op.
	if err := p.Cleanup(ctx, root, ""); err != nil {
		t.Errorf("cleanup of empty path should be no-op, got %v", err)
	}
}
