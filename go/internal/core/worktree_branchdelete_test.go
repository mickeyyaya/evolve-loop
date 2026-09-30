package core

import (
	"context"
	"io"
	"testing"
)

// Uses the gitCall/useFakeGit-style seam from git_seam_test.go (package core
// cannot import test/fixtures.FakeExec — that would be an import cycle).

// branchDeleteFake scripts distinct responses per git subcommand (worktree
// remove vs branch -d), which the single-response gitRec (git_seam_test.go)
// cannot do — this test needs the worktree remove to succeed while branch -d
// fails (the unmerged case).
type branchDeleteFake struct {
	calls           []gitCall
	branchDeleteRC  int
	branchDeleteErr error
}

func (f *branchDeleteFake) run(_ context.Context, name, dir string, args, _ []string, _ io.Reader, _, errw io.Writer) (int, error) {
	f.calls = append(f.calls, gitCall{name: name, dir: dir, args: append([]string(nil), args...)})
	if len(args) > 0 && args[0] == "branch" {
		if f.branchDeleteErr != nil {
			return -1, f.branchDeleteErr
		}
		if errw != nil && f.branchDeleteRC != 0 {
			_, _ = errw.Write([]byte("error: the branch is not fully merged"))
		}
		return f.branchDeleteRC, nil
	}
	return 0, nil // worktree remove always succeeds in these tests
}

func useBranchDeleteFake(t *testing.T, f *branchDeleteFake) {
	t.Helper()
	orig := gitRunner
	gitRunner = f.run
	t.Cleanup(func() { gitRunner = orig })
}

func TestCleanup_DeletesMergedCycleBranch(t *testing.T) {
	f := &branchDeleteFake{branchDeleteRC: 0} // merged → -d succeeds
	useBranchDeleteFake(t, f)

	const projectRoot = "/proj"
	const worktree = "/base/cycle-abcd1234-9"
	const wantBranch = "cycle-abcd1234-9"

	if err := (gitWorktree{}).Cleanup(projectRoot, worktree); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	if len(f.calls) != 2 {
		t.Fatalf("git calls = %+v, want exactly 2 (worktree remove, then branch -d)", f.calls)
	}
	rm := f.calls[0]
	if len(rm.args) < 3 || rm.args[0] != "worktree" || rm.args[1] != "remove" {
		t.Fatalf("first call = %v, want worktree remove first (branch delete must run AFTER)", rm.args)
	}
	del := f.calls[1]
	if len(del.args) < 3 || del.args[0] != "branch" || del.args[1] != "-d" {
		t.Fatalf("second call = %v, want [branch -d %s]", del.args, wantBranch)
	}
	if del.args[2] != wantBranch {
		t.Errorf("branch delete target = %q, want %q (worktree leaf name)", del.args[2], wantBranch)
	}
	if del.dir != projectRoot {
		t.Errorf("branch delete dir = %q, want projectRoot %q (worktree is already gone, can't run -C there)", del.dir, projectRoot)
	}
}

func TestCleanup_UnmergedBranchSurvives_WarnsOnly(t *testing.T) {
	f := &branchDeleteFake{branchDeleteRC: 1} // unmerged → -d refuses
	useBranchDeleteFake(t, f)

	if err := (gitWorktree{}).Cleanup("/proj", "/base/cycle-unmerged-3"); err != nil {
		t.Fatalf("Cleanup: %v, want nil — a refused branch delete is best-effort, not a hard failure", err)
	}

	var sawDelete bool
	for _, c := range f.calls {
		if len(c.args) > 1 && c.args[0] == "branch" {
			sawDelete = true
			if c.args[1] == "-D" {
				t.Fatalf("Cleanup escalated to force delete (-D) after -d refused — git's merged-check must be the only safety net, args=%v", c.args)
			}
			if c.args[1] != "-d" {
				t.Errorf("branch delete flag = %q, want -d", c.args[1])
			}
		}
	}
	if !sawDelete {
		t.Fatalf("Cleanup never attempted a branch delete — calls=%+v", f.calls)
	}
}
