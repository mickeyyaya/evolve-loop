package swarm

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// White-box, fast-tier coverage of mergeWith, the gitexec-backed core behind
// ExecGitMerger.Merge. The real-git path stays covered by
// mergetrain_adversarial_test.go; these pin the exact invocation + the
// conflict-aborts-and-wraps-ErrMergeConflict contract via fixtures.FakeExec.

func TestMergeWith_Success(t *testing.T) {
	t.Parallel()
	fake := &fixtures.FakeExec{} // merge succeeds (zero value)
	g := gitexec.Git{Dir: "/integ", Exec: fake.Run}

	if err := mergeWith(context.Background(), g, "worker-branch"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if keys := fake.CallKeys(); !reflect.DeepEqual(keys, []string{"git merge"}) {
		t.Fatalf("calls = %v, want [git merge]", keys)
	}
	wantArgs := []string{"merge", "--no-ff", "--no-edit", "worker-branch"}
	if got := fake.Calls[0].Args; !reflect.DeepEqual(got, wantArgs) {
		t.Errorf("args = %v, want %v", got, wantArgs)
	}
}

func TestMergeWith_Conflict_AbortsAndWrapsErr(t *testing.T) {
	t.Parallel()
	fake := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git merge": {ExitCode: 1, Stderr: "CONFLICT (content): Merge conflict in foo.go"},
	}}
	g := gitexec.Git{Dir: "/integ", Exec: fake.Run}

	err := mergeWith(context.Background(), g, "worker-branch")
	if !errors.Is(err, ErrMergeConflict) {
		t.Fatalf("err = %v, want it to wrap ErrMergeConflict", err)
	}
	// A failed merge MUST abort so the integration tip is left clean.
	if keys := fake.CallKeys(); !reflect.DeepEqual(keys, []string{"git merge", "git merge"}) {
		t.Fatalf("calls = %v, want [git merge, git merge] (merge then merge --abort)", keys)
	}
	if abortArgs := fake.Calls[1].Args; !reflect.DeepEqual(abortArgs, []string{"merge", "--abort"}) {
		t.Errorf("abort args = %v, want [merge --abort]", abortArgs)
	}
	if !strings.Contains(err.Error(), "CONFLICT") {
		t.Errorf("err = %q, want it to surface git stderr", err)
	}
}
