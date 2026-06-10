package swarm

// mergetrain_adversarial_test.go — cycle-281 test amplification.
// Targets ExecGitMerger.Merge (0%): the production git-shelling merger must
// return an error wrapping ErrMergeConflict when the worktree directory is
// invalid or the merge fails. This is inherently an integration-style test but
// does NOT require a real git repository — an invalid directory produces an
// exec error, which is mapped to ErrMergeConflict.

import (
	"context"
	"errors"
	"testing"
)

// TestExecGitMerger_InvalidWorktreeReturnsConflictError — adversarial: when
// the integration worktree directory does not exist, git exits non-zero and
// ExecGitMerger.Merge must return an error wrapping ErrMergeConflict.
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

// TestExecGitMerger_NotAGitRepoReturnsConflictError — adversarial: a directory
// that exists but is not a git repository causes `git merge` to fail; the
// error must still wrap ErrMergeConflict (the abort path is also exercised,
// returning cleanly).
func TestExecGitMerger_NotAGitRepoReturnsConflictError(t *testing.T) {
	dir := t.TempDir() // exists but is not a git repo
	merger := ExecGitMerger{IntegrationWorktree: dir}
	err := merger.Merge(context.Background(), "integ-branch", "worker-branch")
	if err == nil {
		t.Fatal("Merge in a non-git directory must return an error")
	}
	if !errors.Is(err, ErrMergeConflict) {
		t.Errorf("error must wrap ErrMergeConflict; got %v", err)
	}
}

// TestRunMergeTrain_ResolverRetrySucceeds — adversarial: a conflict resolver
// that unblocks the branch on the second attempt → the worker lands with
// Resolved=true.
func TestRunMergeTrain_ResolverRetrySucceeds(t *testing.T) {
	callCount := 0
	m := &scriptMerger{failBranch: map[string]bool{"cycle-1-w1": true}}
	resolver := func(_ context.Context, workerID, _ string) error {
		callCount++
		// Resolver flips the failure on first call.
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

// TestRunMergeTrain_EmptyWorkerList — adversarial: no workers → zero outcomes
// and no panic. AllMerged is false (no workers were merged).
func TestRunMergeTrain_EmptyWorkerList(t *testing.T) {
	m := &scriptMerger{}
	rep := RunMergeTrain(context.Background(), "integ",
		nil, nil, MergeTrainDeps{Merger: m})
	if len(rep.Outcomes) != 0 {
		t.Errorf("empty worker list must produce zero outcomes; got %+v", rep.Outcomes)
	}
	// AllMerged is false for an empty run (no workers were merged successfully).
	if rep.AllMerged {
		t.Errorf("empty worker list must have AllMerged=false (nothing was merged); got %+v", rep)
	}
}

// TestRunMergeTrain_NilResolverConflictFails — adversarial: conflict with no
// resolver configured means the conflicting worker fails and the train stops.
func TestRunMergeTrain_NilResolverConflictFails(t *testing.T) {
	m := &scriptMerger{failBranch: map[string]bool{"cycle-1-w0": true}}
	rep := RunMergeTrain(context.Background(), "integ",
		[]string{"w0", "w1"}, branchMap("w0", "w1"),
		MergeTrainDeps{Merger: m}) // no Resolver
	if rep.AllMerged {
		t.Fatal("conflict with nil resolver must NOT produce AllMerged=true")
	}
	if len(rep.Outcomes) == 0 || rep.Outcomes[0].Merged {
		t.Errorf("first worker must have failed: %+v", rep.Outcomes)
	}
}
