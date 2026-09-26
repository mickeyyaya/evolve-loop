package swarm

import (
	"context"
	"errors"
	"testing"
)

func TestExecGitMerger_InvalidWorktreeReturnsConflictError(t *testing.T) {
	merger := ExecGitMerger{IntegrationWorktree: "/nonexistent/worktree/that/cannot/exist"}
	err := merger.Merge(context.Background(), "integ-branch", "worker-branch")
	if err == nil {
		t.Fatal("Merge with nonexistent worktree must return an error")
	}
	if !errors.Is(err, ErrMergeConflict) {
		t.Errorf("error must wrap ErrMergeConflict; got %v", err)
	}
}

func TestExecGitMerger_NotAGitRepoReturnsConflictError(t *testing.T) {
	dir := t.TempDir()
	merger := ExecGitMerger{IntegrationWorktree: dir}
	err := merger.Merge(context.Background(), "integ-branch", "worker-branch")
	if err == nil {
		t.Fatal("Merge in a non-git directory must return an error")
	}
	if !errors.Is(err, ErrMergeConflict) {
		t.Errorf("error must wrap ErrMergeConflict; got %v", err)
	}
}

func TestRunMergeTrain_ResolverRetrySucceeds(t *testing.T) {
	callCount := 0
	m := &scriptMerger{failBranch: map[string]bool{"cycle-1-w1": true}}
	resolver := func(_ context.Context, workerID, _ string) error {
		callCount++
		if workerID == "w1" {
			delete(m.failBranch, "cycle-1-w1")
		}
		return nil
	}
	rep := RunMergeTrain(context.Background(), "integ",
		[]string{"w0", "w1", "w2"}, branchMap("w0", "w1", "w2"),
		MergeTrainDeps{Merger: m, Resolver: resolver, MaxRetries: 1})
	if !rep.AllMerged {
		t.Fatalf("resolver resolved the conflict; AllMerged must be true: %+v", rep.Outcomes)
	}
	var w1 *MergeOutcome
	for i := range rep.Outcomes {
		if rep.Outcomes[i].WorkerID == "w1" {
			w1 = &rep.Outcomes[i]
		}
	}
	if w1 == nil || !w1.Resolved {
		t.Errorf("w1 must be Resolved=true after resolver ran; got %+v", w1)
	}
	if callCount == 0 {
		t.Error("resolver must have been called at least once")
	}
}

func TestRunMergeTrain_EmptyWorkerList(t *testing.T) {
	m := &scriptMerger{}
	rep := RunMergeTrain(context.Background(), "integ",
		nil, nil, MergeTrainDeps{Merger: m})
	if len(rep.Outcomes) != 0 {
		t.Errorf("empty worker list must produce zero outcomes; got %+v", rep.Outcomes)
	}
	if rep.AllMerged {
		t.Errorf("empty worker list must have AllMerged=false (nothing was merged); got %+v", rep)
	}
}

func TestRunMergeTrain_NilResolverConflictFails(t *testing.T) {
	m := &scriptMerger{failBranch: map[string]bool{"cycle-1-w0": true}}
	rep := RunMergeTrain(context.Background(), "integ",
		[]string{"w0", "w1"}, branchMap("w0", "w1"),
		MergeTrainDeps{Merger: m})
	if rep.AllMerged {
		t.Fatal("conflict with nil resolver must NOT produce AllMerged=true")
	}
	if len(rep.Outcomes) == 0 || rep.Outcomes[0].Merged {
		t.Errorf("first worker must have failed: %+v", rep.Outcomes)
	}
}
