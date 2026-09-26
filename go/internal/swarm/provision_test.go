package swarm

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/runscope"
)

func integBranchFor(root string, cycle int) string {
	return runscope.New(runscope.LaneFromRoot(root), "", cycle).IntegrationBranch()
}
func workerBranchFor(root string, cycle int, workerID string) string {
	return runscope.New(runscope.LaneFromRoot(root), "", cycle).WorkerBranch(workerID)
}

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
	ctx := context.Background()
	var linked []string
	p := NewGitWorkerProvisioner(func(wt, _ string) { linked = append(linked, wt) }, "")

	integBranch := integBranchFor(root, 5)
	integ, err := p.CreateIntegration(ctx, root, 5)
	if err != nil {
		t.Fatal(err)
	}
	if branchOf(t, integ) != integBranch {
		t.Errorf("integration branch = %q, want %q", branchOf(t, integ), integBranch)
	}

	w0, err := p.CreateWorker(ctx, root, 5, "w0", integBranch)
	if err != nil {
		t.Fatal(err)
	}
	if want := workerBranchFor(root, 5, "w0"); branchOf(t, w0) != want {
		t.Errorf("worker branch = %q, want %q", branchOf(t, w0), want)
	}
	w1, err := p.CreateWorker(ctx, root, 5, "w1", integBranch)
	if err != nil {
		t.Fatal(err)
	}
	if w0 == w1 {
		t.Error("workers must get distinct worktrees")
	}
	if len(linked) != 3 {
		t.Errorf("linkGuardDeps should run per worktree (3), got %d", len(linked))
	}
}

func TestGitWorkerProvisioner_CreateWorkerIdempotent(t *testing.T) {
	root := gitInit(t)
	ctx := context.Background()
	p := NewGitWorkerProvisioner(nil, "")
	integBranch := integBranchFor(root, 1)
	_, _ = p.CreateIntegration(ctx, root, 1)
	a, err := p.CreateWorker(ctx, root, 1, "w0", integBranch)
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.CreateWorker(ctx, root, 1, "w0", integBranch)
	if err != nil {
		t.Fatalf("idempotent re-create failed: %v", err)
	}
	if a != b {
		t.Errorf("idempotent create should return same path: %q vs %q", a, b)
	}
}

func TestGitWorkerProvisioner_Cleanup(t *testing.T) {
	root := gitInit(t)
	ctx := context.Background()
	p := NewGitWorkerProvisioner(nil, "")
	integBranch := integBranchFor(root, 1)
	_, _ = p.CreateIntegration(ctx, root, 1)
	w0, err := p.CreateWorker(ctx, root, 1, "w0", integBranch)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Cleanup(ctx, root, w0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(w0); !os.IsNotExist(err) {
		t.Errorf("worktree should be gone after cleanup, stat err=%v", err)
	}
	if err := p.Cleanup(ctx, root, ""); err != nil {
		t.Errorf("cleanup of empty path should be no-op, got %v", err)
	}
}

func branchExistsSwarm(t *testing.T, root, name string) bool {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "branch", "--list", name).Output()
	if err != nil {
		t.Fatalf("git branch --list %s: %v", name, err)
	}
	return strings.TrimSpace(string(out)) != ""
}

func TestGitWorkerProvisioner_Cleanup_DeletesMergedBranch(t *testing.T) {
	root := gitInit(t)
	ctx := context.Background()
	p := NewGitWorkerProvisioner(nil, "")
	integBranch := integBranchFor(root, 21)
	if _, err := p.CreateIntegration(ctx, root, 21); err != nil {
		t.Fatal(err)
	}
	w0, err := p.CreateWorker(ctx, root, 21, "w0", integBranch)
	if err != nil {
		t.Fatal(err)
	}
	workerBranch := workerBranchFor(root, 21, "w0")
	if !branchExistsSwarm(t, root, workerBranch) {
		t.Fatalf("setup: branch %s should exist right after CreateWorker", workerBranch)
	}

	if err := p.Cleanup(ctx, root, w0); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if branchExistsSwarm(t, root, workerBranch) {
		t.Errorf("branch %s still exists after Cleanup of a merged worker branch — should have been deleted", workerBranch)
	}
}

func TestGitWorkerProvisioner_Cleanup_UnmergedBranchSurvives(t *testing.T) {
	root := gitInit(t)
	ctx := context.Background()
	p := NewGitWorkerProvisioner(nil, "")
	integBranch := integBranchFor(root, 22)
	if _, err := p.CreateIntegration(ctx, root, 22); err != nil {
		t.Fatal(err)
	}
	w0, err := p.CreateWorker(ctx, root, 22, "w0", integBranch)
	if err != nil {
		t.Fatal(err)
	}
	workerBranch := workerBranchFor(root, 22, "w0")

	if err := os.WriteFile(filepath.Join(w0, "unshipped.txt"), []byte("wip"), 0o644); err != nil {
		t.Fatal(err)
	}
	add := exec.Command("git", "-C", w0, "add", ".")
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	commit := exec.Command("git", "-C", w0, "commit", "-q", "-m", "unshipped work")
	commit.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	if err := p.Cleanup(ctx, root, w0); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if !branchExistsSwarm(t, root, workerBranch) {
		t.Errorf("branch %s was deleted despite carrying an unmerged commit — evidence of unshipped work lost", workerBranch)
	}
}

func TestWorktreeBase_AbsoluteOverride(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "custom-base")
	got, err := worktreeBase(custom, "/some/project")
	if err != nil {
		t.Fatalf("absolute override must not error, got %v", err)
	}
	if got != custom {
		t.Errorf("worktreeBase = %q, want %q", got, custom)
	}
}

func TestWorktreeBase_DefaultPath(t *testing.T) {
	got, err := worktreeBase("", "/proj")
	if err != nil {
		t.Fatalf("absolute default must not error, got %v", err)
	}
	if !strings.HasSuffix(got, filepath.Join(".evolve", "worktrees")) {
		t.Errorf("default worktreeBase = %q, must end with .evolve/worktrees", got)
	}
}

func TestWorktreeBase_RelativeOverrideReturnsError(t *testing.T) {
	got, err := worktreeBase("relative-worktrees", "/some/project")
	if err == nil {
		t.Fatalf("worktreeBase with a relative override must return an error, got path %q", got)
	}
	if got != "" {
		t.Errorf("on a relative base worktreeBase must return an empty path, got %q", got)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "absolute") {
		t.Errorf("error = %q, want a message indicating the worktree base must be absolute", err)
	}
}

func TestWorktreeBase_RelativeProjectRootRefused(t *testing.T) {
	got, err := worktreeBase("", ".")
	if err == nil {
		t.Fatalf("worktreeBase(\".\") with the default branch must return an error for a relative project root, got path %q", got)
	}
	if got != "" {
		t.Errorf("on a relative project root worktreeBase must return an empty path, got %q", got)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "absolute") {
		t.Errorf("error = %q, want a message indicating the worktree base/project root must be absolute", err)
	}
}

func TestAddWorktree_RelativeBaseRefused(t *testing.T) {
	root := gitInit(t)

	_, err := NewGitWorkerProvisioner(nil, "relative-worktrees").CreateIntegration(context.Background(), root, 294)
	if err == nil {
		t.Fatal("CreateIntegration with a relative worktree.base override must fail")
	}
	if !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("relative base error = %q, want message mentioning absolute", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "relative-worktrees")); !os.IsNotExist(statErr) {
		t.Fatalf("relative base must be rejected before creating directories, stat err=%v", statErr)
	}
}

func TestCreateWorker_EmptyIntegrationBranch(t *testing.T) {
	root := gitInit(t)
	ctx := context.Background()
	p := NewGitWorkerProvisioner(nil, "")
	wt, err := p.CreateWorker(ctx, root, 9, "w0", "")
	if err != nil {
		t.Fatalf("CreateWorker with empty integrationBranch: %v", err)
	}
	if want := workerBranchFor(root, 9, "w0"); branchOf(t, wt) != want {
		t.Errorf("branch = %q, want %q", branchOf(t, wt), want)
	}
}

func TestAddWorktree_StaleStubRemoved(t *testing.T) {
	root := gitInit(t)
	base := filepath.Join(root, ".evolve", "worktrees")
	ctx := context.Background()

	// The stub sits at the exact path CreateWorker targets, so the teardown path really runs.
	stub := filepath.Join(base, workerBranchFor(root, 7, "w0"))
	if err := os.MkdirAll(stub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stub, "stale.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := NewGitWorkerProvisioner(nil, base)
	wt, err := p.CreateWorker(ctx, root, 7, "w0", "")
	if err != nil {
		t.Fatalf("CreateWorker with stale stub: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt, "stale.txt")); !os.IsNotExist(err) {
		t.Error("stale stub content should have been removed before worktree re-creation")
	}
}
