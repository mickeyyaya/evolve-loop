package core

import (
	"context"
	"errors"
	"reflect"
	"testing"
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
