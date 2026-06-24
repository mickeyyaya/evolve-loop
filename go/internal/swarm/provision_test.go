package swarm

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/runscope"
)

// integBranchFor / workerBranchFor / cycleBranchFor are the runscope-derived
// expected names a test asserts against — the single source the production
// provisioner now mints, so the tests cannot drift from the impl.
func integBranchFor(root string, cycle int) string {
	return runscope.New(runscope.LaneFromRoot(root), "", cycle).IntegrationBranch()
}
func workerBranchFor(root string, cycle int, workerID string) string {
	return runscope.New(runscope.LaneFromRoot(root), "", cycle).WorkerBranch(workerID)
}

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

	// Workers branch off the integration branch with a NAMED branch (symbolic-ref
	// resolvable, required by merge-train/ship).
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
	// linkGuardDeps ran for each provisioned worktree.
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
	b, err := p.CreateWorker(ctx, root, 1, "w0", integBranch) // reuse
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
	// Cleanup of empty path is a no-op.
	if err := p.Cleanup(ctx, root, ""); err != nil {
		t.Errorf("cleanup of empty path should be no-op, got %v", err)
	}
}

// TestWorktreeBase_AbsoluteOverride covers the policy.json worktree.base override
// path. An absolute override is honored verbatim with no error.
func TestWorktreeBase_AbsoluteOverride(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "custom-base") // t.TempDir is absolute
	got, err := worktreeBase(custom, "/some/project")
	if err != nil {
		t.Fatalf("absolute override must not error, got %v", err)
	}
	if got != custom {
		t.Errorf("worktreeBase = %q, want %q", got, custom)
	}
}

// TestWorktreeBase_DefaultPath covers the default (no env) path. The default is
// rooted at the absolute projectRoot, so it returns no error.
func TestWorktreeBase_DefaultPath(t *testing.T) {
	got, err := worktreeBase("", "/proj")
	if err != nil {
		t.Fatalf("absolute default must not error, got %v", err)
	}
	if !strings.HasSuffix(got, filepath.Join(".evolve", "worktrees")) {
		t.Errorf("default worktreeBase = %q, must end with .evolve/worktrees", got)
	}
}

// TestWorktreeBase_RelativeOverrideReturnsError pins the inbox-defect closure
// (swarm-tests-relative-worktree-base): the guard refusing a non-absolute base
// must live in worktreeBase ITSELF — not only one call-site deeper in
// addWorktree. A relative worktree.base override must make worktreeBase return a
// ("", error) whose message identifies the base must be absolute, BEFORE any
// caller touches git/MkdirAll. This is the negative (anti-no-op) axis: a build
// that leaves worktreeBase returning the relative string verbatim fails here.
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

// TestWorktreeBase_RelativeProjectRootRefused pins the LAST gap of the
// swarm-tests-relative-worktree-base inbox defect (cycle-297). Cycle 296 moved
// the IsAbs guard into worktreeBase, but only on the override branch. The
// DEFAULT branch still returned
// filepath.Join(projectRoot, ".evolve", "worktrees") verbatim — which is
// RELATIVE when projectRoot is relative (e.g. "."). A relative worktree base
// breaks `git worktree add` (resolved against an unintended cwd) and the
// tree-diff guard. With no override, worktreeBase("", ".") must return
// ("", error) whose message identifies that the base/root must be absolute,
// BEFORE any caller touches git/MkdirAll. This is the negative (anti-no-op)
// axis: the RED baseline returned ".evolve/worktrees" with a nil error, so this
// test fails until the default branch also guards IsAbs.
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

// TestCreateWorker_EmptyIntegrationBranch covers the empty-integrationBranch fallback.
func TestCreateWorker_EmptyIntegrationBranch(t *testing.T) {
	root := gitInit(t)
	ctx := context.Background()
	p := NewGitWorkerProvisioner(nil, "")
	// Empty integrationBranch → falls back to "HEAD"
	wt, err := p.CreateWorker(ctx, root, 9, "w0", "")
	if err != nil {
		t.Fatalf("CreateWorker with empty integrationBranch: %v", err)
	}
	if want := workerBranchFor(root, 9, "w0"); branchOf(t, wt) != want {
		t.Errorf("branch = %q, want %q", branchOf(t, wt), want)
	}
}

// TestAddWorktree_StaleStubRemoved covers the stale-directory teardown path in
// addWorktree: when the path exists but is NOT a valid git worktree (missing
// .git), git worktree add -B would fail. The impl removes the stub and retries.
func TestAddWorktree_StaleStubRemoved(t *testing.T) {
	root := gitInit(t)
	base := filepath.Join(root, ".evolve", "worktrees")
	ctx := context.Background()

	// Pre-create a stale stub at the EXACT path CreateWorker will target, so the
	// teardown path is actually exercised.
	stub := filepath.Join(base, workerBranchFor(root, 7, "w0"))
	if err := os.MkdirAll(stub, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a dummy file to confirm the stub is not silently kept.
	if err := os.WriteFile(filepath.Join(stub, "stale.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := NewGitWorkerProvisioner(nil, base)
	wt, err := p.CreateWorker(ctx, root, 7, "w0", "")
	if err != nil {
		t.Fatalf("CreateWorker with stale stub: %v", err)
	}
	// The stale file must have been swept away.
	if _, err := os.Stat(filepath.Join(wt, "stale.txt")); !os.IsNotExist(err) {
		t.Error("stale stub content should have been removed before worktree re-creation")
	}
}
