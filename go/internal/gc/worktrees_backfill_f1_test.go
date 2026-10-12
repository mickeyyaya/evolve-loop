package gc

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func failingGitF1(inner sysexec.RunFunc, fails func(dir string, args []string) error) sysexec.RunFunc {
	return func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if err := fails(dir, args); err != nil {
			return 0, err
		}
		return inner(ctx, name, dir, args, env, in, out, errw)
	}
}

func firstArgsF1(args []string, n int) string {
	if len(args) < n {
		n = len(args)
	}
	return strings.Join(args[:n], " ")
}

func TestPlanWorktreesBackfillF1_EachGitStepFailureAbortsThePlanWithThatStep(t *testing.T) {
	boom := errors.New("git unavailable")
	cases := []struct {
		name string
		fail string
		want string
	}{
		{"worktree list", "worktree list", "gc: git worktree list --porcelain"},
		{"merged branches", "branch --merged", "gc: git branch --merged HEAD"},
		{"orphan branch listing", "branch --list", "gc: git branch --list cycle-*"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newWorktreesTestEnv(t)
			e.addWorktree("cycle-aaa1111-570", "cycle-aaa1111-570", 20*time.Hour, false, true)
			o := e.opts()
			o.Exec = failingGitF1(e.git.run, func(_ string, args []string) error {
				if firstArgsF1(args, 2) == tc.fail {
					return boom
				}
				return nil
			})
			m, err := PlanWorktrees(o)
			if !errors.Is(err, boom) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if len(m.Items) != 0 {
				t.Fatalf("a failed plan must not return partial items: %+v", m.Items)
			}
		})
	}
}

func TestPlanWorktreesBackfillF1_RefusesToPlanWithoutAnExecSeam(t *testing.T) {
	t.Parallel()
	m, err := PlanWorktrees(WorktreeOptions{})
	if err == nil || !strings.Contains(err.Error(), "PlanWorktrees requires Exec") || len(m.Items) != 0 {
		t.Fatalf("manifest=%+v err=%v", m, err)
	}
}

func TestApplyWorktreesBackfillF1_RefusesToApplyWithoutAnExecSeam(t *testing.T) {
	t.Parallel()
	err := ApplyWorktrees(WorktreeOptions{}, WorktreeManifest{})
	if err == nil || !strings.Contains(err.Error(), "ApplyWorktrees requires Exec") {
		t.Fatalf("err = %v", err)
	}
}

func TestApplyWorktreesBackfillF1_AShipLockThatCannotBeTakenStopsBeforeAnyGitCall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".evolve"), []byte("a file where the evolve dir must go"), 0o644); err != nil {
		t.Fatal(err)
	}
	calls := 0
	o := WorktreeOptions{ProjectRoot: root, Exec: func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		calls++
		return 0, nil
	}}
	err := ApplyWorktrees(o, WorktreeManifest{Items: []WorktreeItem{{Path: filepath.Join(root, "wt"), Action: WorktreeActionRemove}}})
	if err == nil || !strings.Contains(err.Error(), "gc: acquire ship.lock:") || calls != 0 {
		t.Fatalf("err = %v, git calls = %d", err, calls)
	}
}

func TestApplyWorktreesBackfillF1_AFailedPruneAfterARealRemovalIsReported(t *testing.T) {
	e := newWorktreesTestEnv(t)
	wt := e.addWorktree("cycle-aaa1111-570", "cycle-aaa1111-570", 20*time.Hour, false, true)
	o := e.opts()
	m, err := PlanWorktrees(o)
	if err != nil {
		t.Fatal(err)
	}
	boom := errors.New("prune failed")
	o.Exec = failingGitF1(e.git.run, func(_ string, args []string) error {
		if firstArgsF1(args, 2) == "worktree prune" {
			return boom
		}
		return nil
	})
	err = ApplyWorktrees(o, m)
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "gc: git worktree prune") {
		t.Fatalf("err = %v, want the prune failure", err)
	}
	if n := e.git.callCount("worktree remove " + wt); n != 1 {
		t.Fatalf("the removal ran %d times, want exactly once before the prune", n)
	}
}

func TestApplyWorktreesBackfillF1_ARemovalGitRefusesIsReportedAndSkipsThePrune(t *testing.T) {
	e := newWorktreesTestEnv(t)
	e.addWorktree("cycle-aaa1111-570", "cycle-aaa1111-570", 20*time.Hour, false, true)
	o := e.opts()
	m, err := PlanWorktrees(o)
	if err != nil {
		t.Fatal(err)
	}
	boom := errors.New("worktree locked")
	o.Exec = failingGitF1(e.git.run, func(_ string, args []string) error {
		if firstArgsF1(args, 2) == "worktree remove" {
			return boom
		}
		return nil
	})
	err = ApplyWorktrees(o, m)
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "gc: git worktree remove") {
		t.Fatalf("err = %v, want the removal failure", err)
	}
	if n := e.git.callCount("worktree prune"); n != 0 {
		t.Fatalf("prune ran %d times although nothing was removed", n)
	}
}

func TestApplyWorktreesBackfillF1_ATargetWhoseStatusCannotBeRecheckedIsRefusedUntouched(t *testing.T) {
	e := newWorktreesTestEnv(t)
	wt := e.addWorktree("cycle-aaa1111-570", "cycle-aaa1111-570", 20*time.Hour, false, true)
	o := e.opts()
	m, err := PlanWorktrees(o)
	if err != nil {
		t.Fatal(err)
	}
	boom := errors.New("status exploded")
	o.Exec = failingGitF1(e.git.run, func(dir string, args []string) error {
		if len(args) > 0 && args[0] == "status" && dir == wt {
			return boom
		}
		return nil
	})
	err = ApplyWorktrees(o, m)
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "gc: refuse "+wt+": status re-check failed") {
		t.Fatalf("err = %v", err)
	}
	for _, forbidden := range []string{"worktree remove", "branch -d"} {
		if n := e.git.callCount(forbidden); n != 0 {
			t.Errorf("%q ran %d times for a refused target", forbidden, n)
		}
	}
}

func TestIsLiveBackfillF1(t *testing.T) {
	t.Run("a stray file in the runs directory is not a run", func(t *testing.T) {
		e := newWorktreesTestEnv(t)
		wt := filepath.Join(e.worktreeBase, "cycle-aaa-9")
		writeFile(t, filepath.Join(e.evolveDir, "runs", "stray.txt"), "x")
		if e.opts().isLive(wt) {
			t.Fatal("a regular file under runs/ must not make a worktree live")
		}
	})
	t.Run("an unparsable cycle-state does not make a worktree live", func(t *testing.T) {
		e := newWorktreesTestEnv(t)
		wt := filepath.Join(e.worktreeBase, "cycle-aaa-9")
		writeFile(t, filepath.Join(e.evolveDir, "cycle-state.json"), "{not json")
		if e.opts().isLive(wt) {
			t.Fatal("garbage in cycle-state.json proves nothing")
		}
	})
	t.Run("an open run pointing at the worktree keeps it live", func(t *testing.T) {
		e := newWorktreesTestEnv(t)
		wt := filepath.Join(e.worktreeBase, "cycle-aaa-9")
		writeFile(t, filepath.Join(e.evolveDir, "runs", "scratch", "run.json"), `{"active_worktree":"`+wt+`"}`)
		if !e.opts().isLive(wt) {
			t.Fatal("a run directory whose run.json names the worktree and is not closed out is live")
		}
	})
	t.Run("a different worktree named by the state is not this one", func(t *testing.T) {
		e := newWorktreesTestEnv(t)
		writeFile(t, filepath.Join(e.evolveDir, "cycle-state.json"), `{"active_worktree":"/elsewhere/cycle-bbb-1"}`)
		if e.opts().isLive(filepath.Join(e.worktreeBase, "cycle-aaa-9")) {
			t.Fatal("another worktree's state must not pin this one")
		}
	})
}

func TestFinishedForBackfillF1(t *testing.T) {
	e := newWorktreesTestEnv(t)
	o := e.opts()
	if d, ok := o.finishedFor(filepath.Join(e.worktreeBase, "cycle-noid")); ok || d != 0 {
		t.Errorf("a leaf with no cycle number has no finish time, got (%v, %v)", d, ok)
	}
	if d, ok := o.finishedFor(filepath.Join(e.worktreeBase, "cycle-aaa-41")); ok || d != 0 {
		t.Errorf("a cycle that never closed out has no finish time, got (%v, %v)", d, ok)
	}
	closeOutCycle(t, e.projectRoot, 42, e.now.Add(-36*time.Hour))
	if d, ok := o.finishedFor(filepath.Join(e.worktreeBase, "cycle-aaa-42")); !ok || d != 36*time.Hour {
		t.Errorf("a closed-out cycle reports its age, got (%v, %v)", d, ok)
	}
}
