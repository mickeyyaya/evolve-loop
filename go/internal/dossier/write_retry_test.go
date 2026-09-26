package dossier

import (
	"context"
	"io"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

// fakeGitRun is a scripted sysexec.RunFunc for commitPairGit's retry tests.
type fakeGitRun struct {
	calls      []string // recorded subcommands, in call order ("add", "diff", "commit")
	failCommit int      // number of leading "commit" calls that must fail before succeeding
	commitErr  string   // stderr text for a failing commit; defaults to an index.lock message
}

func (f *fakeGitRun) run(_ context.Context, _, _ string, args, _ []string, _ io.Reader, _, stderr io.Writer) (int, error) {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	f.calls = append(f.calls, sub)
	switch sub {
	case "diff":
		return 1, nil // non-zero == differences ARE staged, so commitPairGit must proceed to commit
	case "commit":
		if f.failCommit > 0 {
			f.failCommit--
			msg := f.commitErr
			if msg == "" {
				msg = "fatal: Unable to create '.git/index.lock': File exists."
			}
			if stderr != nil {
				_, _ = stderr.Write([]byte(msg))
			}
			return 128, nil
		}
		return 0, nil
	default: // "add"
		return 0, nil
	}
}

func (f *fakeGitRun) commitCalls() int {
	n := 0
	for _, c := range f.calls {
		if c == "commit" {
			n++
		}
	}
	return n
}

func TestCommitPairGit_RetriesTransientLockFailure(t *testing.T) {
	f := &fakeGitRun{failCommit: 2} // 2 transient failures, 3rd attempt succeeds
	g := gitexec.Git{Dir: t.TempDir(), Exec: f.run}

	if err := commitPairGit(g, "cycle-99"); err != nil {
		t.Fatalf("commitPairGit: %v, want nil (retry must recover from transient lock contention)", err)
	}
	if got := f.commitCalls(); got != 3 {
		t.Errorf("commit attempts = %d, want 3 (2 transient failures + 1 success)", got)
	}
}

func TestCommitPairGit_UnrecoverableAfterBoundedRetries(t *testing.T) {
	f := &fakeGitRun{failCommit: 1000} // never succeeds within any reasonable bound
	g := gitexec.Git{Dir: t.TempDir(), Exec: f.run}

	err := commitPairGit(g, "cycle-100")
	if err == nil {
		t.Fatal("commitPairGit: want error after persistent lock contention, got nil")
	}
	attempts := f.commitCalls()
	if attempts < 2 {
		t.Errorf("commit attempts = %d, want >=2 (must actually retry once before giving up, not fail on the first attempt)", attempts)
	}
	if attempts > 10 {
		t.Errorf("commit attempts = %d, want a SMALL bounded retry count (unbounded retry would hang cycle finalization)", attempts)
	}
}

func TestCommitPairGit_NonTransientErrorFailsFast(t *testing.T) {
	f := &fakeGitRun{failCommit: 1000, commitErr: "fatal: not a git repository (or any of the parent directories): .git"}
	g := gitexec.Git{Dir: t.TempDir(), Exec: f.run}

	err := commitPairGit(g, "cycle-101")
	if err == nil {
		t.Fatal("commitPairGit: want error for a permanent git failure, got nil")
	}
	if got := f.commitCalls(); got != 1 {
		t.Errorf("commit attempts = %d, want exactly 1 (non-transient errors must not be retried)", got)
	}
}
