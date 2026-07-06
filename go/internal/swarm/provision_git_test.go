package swarm

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// White-box, fast-tier coverage that provision's git calls route through the
// gitexec seam (the newGit factory lets each call pick its own -C dir). The
// real-git reuse/teardown/error paths stay covered by provision_test.go.

func fakeProvisioner(fake *fixtures.FakeExec, baseOverride string) gitWorkerProvisioner {
	return gitWorkerProvisioner{baseOverride: baseOverride, newGit: func(dir string) gitexec.Git {
		return gitexec.Git{Dir: dir, Exec: fake.Run}
	}}
}

func TestCleanup_RoutesGitWorktreeRemoveThroughSeam(t *testing.T) {
	t.Parallel()
	fake := &fixtures.FakeExec{}
	p := fakeProvisioner(fake, "")

	if err := p.Cleanup(context.Background(), "/repo", t.TempDir()); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if keys := fake.CallKeys(); !reflect.DeepEqual(keys, []string{"git worktree"}) {
		t.Fatalf("calls = %v, want [git worktree]", keys)
	}
	got := fake.Calls[0].Args
	if len(got) < 3 || got[0] != "worktree" || got[1] != "remove" || got[2] != "--force" {
		t.Errorf("args = %v, want [worktree remove --force <wt>]", got)
	}
}

// TestCleanup_RoutesGitBranchDeleteThroughSeam (S3, workspace-hygiene plan):
// Cleanup must run `git branch -d <leaf>` in projectRoot AFTER the worktree
// remove, through the same injectable seam — the FakeExec fast-tier mirror of
// TestGitWorkerProvisioner_Cleanup_DeletesMergedBranch's real-git assertion.
func TestCleanup_RoutesGitBranchDeleteThroughSeam(t *testing.T) {
	fake := &fixtures.FakeExec{}
	p := fakeProvisioner(fake, "")

	const worktree = "/base/cycle-9-w0"
	if err := p.Cleanup(context.Background(), "/repo", worktree); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if keys := fake.CallKeys(); !reflect.DeepEqual(keys, []string{"git worktree", "git branch"}) {
		t.Fatalf("calls = %v, want [git worktree, git branch] (remove THEN branch delete)", keys)
	}
	del := fake.Calls[1]
	if del.Dir != "/repo" {
		t.Errorf("branch delete dir = %q, want projectRoot %q", del.Dir, "/repo")
	}
	wantArgs := []string{"branch", "-d", "cycle-9-w0"}
	if !reflect.DeepEqual(del.Args, wantArgs) {
		t.Errorf("branch delete args = %v, want %v", del.Args, wantArgs)
	}
}

// TestCleanup_UnmergedBranch_NeverForceDeletes (S3): a scripted `branch -d`
// refusal (rc=1, the unmerged case) must NOT be escalated to `-D`, and
// Cleanup must still return nil (best-effort).
func TestCleanup_UnmergedBranch_NeverForceDeletes(t *testing.T) {
	fake := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git branch": {ExitCode: 1, Stderr: "error: the branch is not fully merged"},
	}}
	p := fakeProvisioner(fake, "")

	if err := p.Cleanup(context.Background(), "/repo", "/base/cycle-9-w1"); err != nil {
		t.Fatalf("Cleanup: %v, want nil — a refused branch delete is best-effort", err)
	}
	for _, c := range fake.Calls {
		if c.Key == "git branch" && len(c.Args) > 1 && c.Args[1] == "-D" {
			t.Fatalf("Cleanup escalated to force delete (-D) after -d refused, args=%v", c.Args)
		}
	}
}

func TestAddWorktree_FreshAdd_RoutesGitWorktreeAddThroughSeam(t *testing.T) {
	base := t.TempDir()
	fake := &fixtures.FakeExec{}
	p := fakeProvisioner(fake, base)

	wt, err := p.CreateIntegration(context.Background(), "/repo", 5)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	integBranch := integBranchFor("/repo", 5)
	if want := filepath.Join(base, integBranch); wt != want {
		t.Errorf("wt = %q, want %q", wt, want)
	}
	// Fresh add: the worktree dir does not exist, so no reuse rev-parse probe —
	// exactly one git call, the worktree add.
	if keys := fake.CallKeys(); !reflect.DeepEqual(keys, []string{"git worktree"}) {
		t.Fatalf("calls = %v, want [git worktree]", keys)
	}
	wantArgs := []string{"worktree", "add", "-B", integBranch, wt, "HEAD"}
	if got := fake.Calls[0].Args; !reflect.DeepEqual(got, wantArgs) {
		t.Errorf("args = %v, want %v", got, wantArgs)
	}
}
