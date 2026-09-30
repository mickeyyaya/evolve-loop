package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func TestPruneSupersededOrphans_SupersededNoPRIsPruned(t *testing.T) {
	s := &seamGit{respond: func(args []string) (string, int) {
		switch {
		case has(args, "for-each-ref"):
			return "cycle-1\n", 0
		case has(args, "merge-base", "--is-ancestor"):
			return "", 0 // superseded (already landed)
		case has(args, "branch", "-D"):
			return "Deleted branch cycle-1\n", 0
		}
		return "", 0
	}}
	useSeamGit(t, s)

	verdicts, err := PruneSupersededOrphans(context.Background(), "/wt", "main",
		func(string) (bool, error) { return false, nil }) // no open PR
	if err != nil {
		t.Fatalf("prune err %v, want nil", err)
	}
	want := []OrphanVerdict{{Ref: "cycle-1", Superseded: true, Pruned: true}}
	if !reflect.DeepEqual(verdicts, want) {
		t.Errorf("verdicts = %+v, want %+v", verdicts, want)
	}
	if !s.calledWith("branch", "-D", "cycle-1") {
		t.Error("superseded PR-free branch was not deleted — prune never issued `git branch -D`")
	}
}

func TestPruneSupersededOrphans_OpenPRFlaggedButKept(t *testing.T) {
	s := &seamGit{respond: func(args []string) (string, int) {
		switch {
		case has(args, "for-each-ref"):
			return "cycle-2\n", 0
		case has(args, "merge-base", "--is-ancestor"):
			return "", 0 // superseded
		}
		return "", 0
	}}
	useSeamGit(t, s)

	verdicts, err := PruneSupersededOrphans(context.Background(), "/wt", "main",
		func(string) (bool, error) { return true, nil }) // open PR present
	if err != nil {
		t.Fatalf("prune err %v, want nil", err)
	}
	v := verdicts[0]
	if !v.Superseded || v.Pruned {
		t.Errorf("verdict = %+v, want superseded=true pruned=false (kept behind open PR)", v)
	}
	if s.calledWith("branch", "-D") {
		t.Error("deleted a branch with an open PR — violates verify_remote_pr_before_branch_delete")
	}
}

func TestPruneSupersededOrphans_DistinctBranchUntouched(t *testing.T) {
	s := &seamGit{respond: func(args []string) (string, int) {
		switch {
		case has(args, "for-each-ref"):
			return "cycle-3\n", 0
		case has(args, "merge-base", "--is-ancestor"):
			return "", 1 // not an ancestor
		case has(args, "cherry"):
			return "+ feed1234\n", 0 // has a commit not on base → distinct work
		}
		return "", 0
	}}
	useSeamGit(t, s)

	prCalled := false
	verdicts, err := PruneSupersededOrphans(context.Background(), "/wt", "main",
		func(string) (bool, error) { prCalled = true; return false, nil })
	if err != nil {
		t.Fatalf("prune err %v, want nil", err)
	}
	v := verdicts[0]
	if v.Superseded || v.Pruned {
		t.Errorf("verdict = %+v, want superseded=false pruned=false for distinct work", v)
	}
	if prCalled {
		t.Error("hasOpenPR consulted for a non-superseded branch — the PR check should gate only supersession")
	}
	if s.calledWith("branch", "-D") {
		t.Error("deleted a distinct, not-yet-landed branch")
	}
}

func TestPruneSupersededOrphans_HasOpenPRErrorAborts(t *testing.T) {
	s := &seamGit{respond: func(args []string) (string, int) {
		switch {
		case has(args, "for-each-ref"):
			return "cycle-4\n", 0
		case has(args, "merge-base", "--is-ancestor"):
			return "", 0 // superseded → hasOpenPR consulted
		}
		return "", 0
	}}
	useSeamGit(t, s)

	_, err := PruneSupersededOrphans(context.Background(), "/wt", "main",
		func(string) (bool, error) { return false, errors.New("gh down") })
	if err == nil {
		t.Fatal("prune succeeded despite a hasOpenPR error, want the walk to abort")
	}
	if s.calledWith("branch", "-D") {
		t.Error("issued a delete after a hasOpenPR error — must abort before mutating refs")
	}
}

// worktreePorcelain renders `git worktree list --porcelain` for a main tree on
// main plus one linked worktree per checked-out branch.
func worktreePorcelain(branches ...string) string {
	var b strings.Builder
	b.WriteString("worktree /repo\nHEAD 1111111111111111111111111111111111111111\nbranch refs/heads/main\n\n")
	for i, br := range branches {
		fmt.Fprintf(&b, "worktree /repo/.evolve/worktrees/lane-%d\nHEAD 2222222222222222222222222222222222222222\nbranch refs/heads/%s\n\n", i, br)
	}
	return b.String()
}

// bindBranch records a continuation binding naming branch in root's registry.
func bindBranch(t *testing.T, root, scope, branch string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := continuation.WriteRegistryEntry(root, scope, continuation.Continuation{Branch: branch, SnapshotSHA: "3333333333333333333333333333333333333333"}); err != nil {
		t.Fatalf("bind %s: %v", branch, err)
	}
}

// recordPRChecks answers "no open PR" and records every ref it was asked about.
func recordPRChecks(consulted *[]string) func(string) (bool, error) {
	return func(ref string) (bool, error) {
		*consulted = append(*consulted, ref)
		return false, nil
	}
}

func TestPruneSupersededOrphans_CheckedOutRefSkippedNotAborted(t *testing.T) {
	s := &seamGit{respond: func(args []string) (string, int) {
		switch {
		case has(args, "for-each-ref"):
			return "cycle-1\ncycle-2\n", 0
		case has(args, "worktree", "list", "--porcelain"):
			return worktreePorcelain("cycle-1"), 0
		case has(args, "merge-base", "--is-ancestor"):
			return "", 0 // both superseded
		case has(args, "branch", "-D", "cycle-1"):
			return "", 1 // real git refuses a branch checked out in a worktree
		case has(args, "branch", "-D", "cycle-2"):
			return "Deleted branch cycle-2\n", 0
		}
		return "", 0
	}}
	useSeamGit(t, s)

	var consulted []string
	verdicts, err := PruneSupersededOrphans(context.Background(), "/wt", "main", recordPRChecks(&consulted))
	if err != nil {
		t.Fatalf("prune err %v, want the checked-out cycle-1 kept and the walk to reach cycle-2", err)
	}
	want := []OrphanVerdict{
		{Ref: "cycle-1", Superseded: true, KeptCheckedOut: true},
		{Ref: "cycle-2", Superseded: true, Pruned: true},
	}
	if !reflect.DeepEqual(verdicts, want) {
		t.Errorf("verdicts = %+v, want %+v", verdicts, want)
	}
	if s.calledWith("branch", "-D", "cycle-1") {
		t.Error("issued `git branch -D` for a ref checked out in a worktree — it must be kept before any delete")
	}
	if slices.Contains(consulted, "cycle-1") {
		t.Error("hasOpenPR consulted for a checked-out ref — the checked-out keep must come before the PR check")
	}
	if !s.calledWith("branch", "-D", "cycle-2") {
		t.Error("walk never deleted cycle-2 after keeping the checked-out cycle-1")
	}
}

func TestPruneSupersededOrphans_MultipleConsecutiveCheckedOutRefsAllSkipped(t *testing.T) {
	s := &seamGit{respond: func(args []string) (string, int) {
		switch {
		case has(args, "for-each-ref"):
			return "cycle-1\ncycle-2\ncycle-3\n", 0
		case has(args, "worktree", "list", "--porcelain"):
			return worktreePorcelain("cycle-1", "cycle-2"), 0
		case has(args, "merge-base", "--is-ancestor"):
			return "", 0
		case has(args, "branch", "-D", "cycle-1"), has(args, "branch", "-D", "cycle-2"):
			return "", 1
		case has(args, "branch", "-D", "cycle-3"):
			return "Deleted branch cycle-3\n", 0
		}
		return "", 0
	}}
	useSeamGit(t, s)

	verdicts, err := PruneSupersededOrphans(context.Background(), "/wt", "main",
		func(string) (bool, error) { return false, nil })
	if err != nil {
		t.Fatalf("prune err %v, want both checked-out refs kept and cycle-3 reached", err)
	}
	want := []OrphanVerdict{
		{Ref: "cycle-1", Superseded: true, KeptCheckedOut: true},
		{Ref: "cycle-2", Superseded: true, KeptCheckedOut: true},
		{Ref: "cycle-3", Superseded: true, Pruned: true},
	}
	if !reflect.DeepEqual(verdicts, want) {
		t.Errorf("verdicts = %+v, want %+v", verdicts, want)
	}
	if s.calledWith("branch", "-D", "cycle-1") || s.calledWith("branch", "-D", "cycle-2") {
		t.Error("issued `git branch -D` for a checked-out ref")
	}
}

// The porcelain carries a detached worktree and a locked one; cycle-1 is a
// name prefix of the checked-out cycle-10 and must not be mistaken for it.
func TestPruneSupersededOrphans_CheckedOutMatchIsExactBranchName(t *testing.T) {
	porcelain := "worktree /repo\nHEAD 1111111111111111111111111111111111111111\nbranch refs/heads/main\n\n" +
		"worktree /repo/.evolve/worktrees/det\nHEAD 2222222222222222222222222222222222222222\ndetached\n\n" +
		"worktree /repo/.evolve/worktrees/lane\nHEAD 4444444444444444444444444444444444444444\nbranch refs/heads/cycle-10\nlocked in use\n\n"
	s := &seamGit{respond: func(args []string) (string, int) {
		switch {
		case has(args, "for-each-ref"):
			return "cycle-1\ncycle-10\n", 0
		case has(args, "worktree", "list", "--porcelain"):
			return porcelain, 0
		case has(args, "merge-base", "--is-ancestor"):
			return "", 0
		case has(args, "branch", "-D", "cycle-1"):
			return "Deleted branch cycle-1\n", 0
		case has(args, "branch", "-D", "cycle-10"):
			return "", 1
		}
		return "", 0
	}}
	useSeamGit(t, s)

	verdicts, err := PruneSupersededOrphans(context.Background(), "/wt", "main",
		func(string) (bool, error) { return false, nil })
	if err != nil {
		t.Fatalf("prune err %v, want nil", err)
	}
	want := []OrphanVerdict{
		{Ref: "cycle-1", Superseded: true, Pruned: true},
		{Ref: "cycle-10", Superseded: true, KeptCheckedOut: true},
	}
	if !reflect.DeepEqual(verdicts, want) {
		t.Errorf("verdicts = %+v, want %+v (only the exact checked-out branch is kept)", verdicts, want)
	}
}

func TestPruneSupersededOrphans_ContinuationBoundIsKept(t *testing.T) {
	root := t.TempDir()
	bindBranch(t, root, "scope-bound", "cycle-7")
	s := &seamGit{respond: func(args []string) (string, int) {
		switch {
		case has(args, "for-each-ref"):
			return "cycle-7\ncycle-8\n", 0
		case has(args, "worktree", "list", "--porcelain"):
			return worktreePorcelain(), 0
		case has(args, "merge-base", "--is-ancestor"):
			return "", 0 // both superseded
		case has(args, "branch", "-D"):
			return "Deleted branch\n", 0
		}
		return "", 0
	}}
	useSeamGit(t, s)

	var consulted []string
	verdicts, err := PruneSupersededOrphans(context.Background(), root, "main", recordPRChecks(&consulted))
	if err != nil {
		t.Fatalf("prune err %v, want nil", err)
	}
	want := []OrphanVerdict{
		{Ref: "cycle-7", Superseded: true, KeptBound: true},
		{Ref: "cycle-8", Superseded: true, Pruned: true},
	}
	if !reflect.DeepEqual(verdicts, want) {
		t.Errorf("verdicts = %+v, want %+v", verdicts, want)
	}
	if s.calledWith("branch", "-D", "cycle-7") {
		t.Error("deleted a branch a continuation binding names — its preserved work would be unrecoverable")
	}
	if slices.Contains(consulted, "cycle-7") {
		t.Error("hasOpenPR consulted for a continuation-bound ref — the binding keep must come before the PR check")
	}
	if !s.calledWith("branch", "-D", "cycle-8") {
		t.Error("walk never deleted the unbound cycle-8")
	}
}

func TestPruneSupersededOrphans_CheckedOutAndBoundReportsOneReason(t *testing.T) {
	root := t.TempDir()
	bindBranch(t, root, "scope-both", "cycle-9")
	s := &seamGit{respond: func(args []string) (string, int) {
		switch {
		case has(args, "for-each-ref"):
			return "cycle-9\n", 0
		case has(args, "worktree", "list", "--porcelain"):
			return worktreePorcelain("cycle-9"), 0
		case has(args, "merge-base", "--is-ancestor"):
			return "", 0
		case has(args, "branch", "-D"):
			return "", 1
		}
		return "", 0
	}}
	useSeamGit(t, s)

	verdicts, err := PruneSupersededOrphans(context.Background(), root, "main",
		func(string) (bool, error) { return false, nil })
	if err != nil {
		t.Fatalf("prune err %v, want nil", err)
	}
	if len(verdicts) != 1 {
		t.Fatalf("verdicts = %+v, want exactly one", verdicts)
	}
	v := verdicts[0]
	if v.Pruned || v.KeptDeleteFailed || v.KeptCheckedOut == v.KeptBound {
		t.Errorf("verdict = %+v, want exactly one of KeptCheckedOut/KeptBound set and nothing else", v)
	}
	if s.calledWith("branch", "-D") {
		t.Error("issued `git branch -D` for a ref that is both checked out and continuation-bound")
	}
}

// Exit 128 is a fatal git error that has nothing to do with worktrees.
func TestPruneSupersededOrphans_DeleteFailureReportedPerRefNotAborted(t *testing.T) {
	s := &seamGit{respond: func(args []string) (string, int) {
		switch {
		case has(args, "for-each-ref"):
			return "cycle-1\ncycle-2\n", 0
		case has(args, "worktree", "list", "--porcelain"):
			return worktreePorcelain(), 0
		case has(args, "merge-base", "--is-ancestor"):
			return "", 0
		case has(args, "branch", "-D", "cycle-1"):
			return "", 128
		case has(args, "branch", "-D", "cycle-2"):
			return "Deleted branch cycle-2\n", 0
		}
		return "", 0
	}}
	useSeamGit(t, s)

	verdicts, err := PruneSupersededOrphans(context.Background(), "/wt", "main",
		func(string) (bool, error) { return false, nil })
	if err != nil {
		t.Fatalf("prune err %v, want the failed delete reported on cycle-1 and the walk to reach cycle-2", err)
	}
	want := []OrphanVerdict{
		{Ref: "cycle-1", Superseded: true, KeptDeleteFailed: true},
		{Ref: "cycle-2", Superseded: true, Pruned: true},
	}
	if !reflect.DeepEqual(verdicts, want) {
		t.Errorf("verdicts = %+v, want %+v (a delete failure on a ref no worktree holds is its own reason, never checked-out and never silent)", verdicts, want)
	}
}

func TestPruneSupersededOrphans_WorktreeListFailureAbortsBeforeAnyDelete(t *testing.T) {
	s := &seamGit{respond: func(args []string) (string, int) {
		switch {
		case has(args, "for-each-ref"):
			return "cycle-1\ncycle-2\n", 0
		case has(args, "worktree", "list"):
			return "", 128
		case has(args, "merge-base", "--is-ancestor"):
			return "", 0
		}
		return "", 0
	}}
	useSeamGit(t, s)

	_, err := PruneSupersededOrphans(context.Background(), "/wt", "main",
		func(string) (bool, error) { return false, nil })
	if err == nil {
		t.Fatal("prune succeeded although `git worktree list` failed, want the walk to abort — checked-out refs cannot be told apart")
	}
	if s.calledWith("branch", "-D") {
		t.Error("issued `git branch -D` without knowing which refs are checked out")
	}
}

func TestPruneSupersededOrphans_CorruptRegistryAbortsBeforeAnyDelete(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(continuation.RegistryPath(root), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &seamGit{respond: func(args []string) (string, int) {
		switch {
		case has(args, "for-each-ref"):
			return "cycle-1\ncycle-2\n", 0
		case has(args, "worktree", "list", "--porcelain"):
			return worktreePorcelain(), 0
		case has(args, "merge-base", "--is-ancestor"):
			return "", 0
		}
		return "", 0
	}}
	useSeamGit(t, s)

	_, err := PruneSupersededOrphans(context.Background(), root, "main",
		func(string) (bool, error) { return false, nil })
	if err == nil {
		t.Fatal("prune succeeded over an unreadable continuation registry, want the walk to abort — bound refs cannot be told apart")
	}
	if s.calledWith("branch", "-D") {
		t.Error("issued `git branch -D` without knowing which refs are continuation-bound")
	}
}

// The keep must not widen into swallowing unrelated errors: a later hasOpenPR
// failure still aborts the walk.
func TestPruneSupersededOrphans_HasOpenPRErrorStillAbortsAfterCheckedOutSkip(t *testing.T) {
	s := &seamGit{respond: func(args []string) (string, int) {
		switch {
		case has(args, "for-each-ref"):
			return "cycle-4\ncycle-5\n", 0
		case has(args, "worktree", "list", "--porcelain"):
			return worktreePorcelain("cycle-4"), 0
		case has(args, "merge-base", "--is-ancestor"):
			return "", 0
		case has(args, "branch", "-D", "cycle-4"):
			return "", 1
		}
		return "", 0
	}}
	useSeamGit(t, s)

	_, err := PruneSupersededOrphans(context.Background(), "/wt", "main",
		func(ref string) (bool, error) {
			if ref == "cycle-5" {
				return false, errors.New("gh down")
			}
			return false, nil
		})
	if err == nil {
		t.Fatal("prune succeeded despite a hasOpenPR error on cycle-5, want the walk to abort")
	}
	if s.calledWith("branch", "-D") {
		t.Error("issued a delete — cycle-4 is checked out and cycle-5's PR check failed")
	}
}
