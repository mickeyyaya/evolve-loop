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

func fakeProvisioner(fake *fixtures.FakeExec) gitWorkerProvisioner {
	return gitWorkerProvisioner{newGit: func(dir string) gitexec.Git {
		return gitexec.Git{Dir: dir, Exec: fake.Run}
	}}
}

func TestCleanup_RoutesGitWorktreeRemoveThroughSeam(t *testing.T) {
	t.Parallel()
	fake := &fixtures.FakeExec{}
	p := fakeProvisioner(fake)

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

func TestAddWorktree_FreshAdd_RoutesGitWorktreeAddThroughSeam(t *testing.T) {
	base := t.TempDir()
	t.Setenv("EVOLVE_WORKTREE_BASE", base)
	fake := &fixtures.FakeExec{}
	p := fakeProvisioner(fake)

	wt, err := p.CreateIntegration(context.Background(), "/repo", 5)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if want := filepath.Join(base, "cycle-5-integration"); wt != want {
		t.Errorf("wt = %q, want %q", wt, want)
	}
	// Fresh add: the worktree dir does not exist, so no reuse rev-parse probe —
	// exactly one git call, the worktree add.
	if keys := fake.CallKeys(); !reflect.DeepEqual(keys, []string{"git worktree"}) {
		t.Fatalf("calls = %v, want [git worktree]", keys)
	}
	wantArgs := []string{"worktree", "add", "-B", "cycle-5-integration", wt, "HEAD"}
	if got := fake.Calls[0].Args; !reflect.DeepEqual(got, wantArgs) {
		t.Errorf("args = %v, want %v", got, wantArgs)
	}
}
