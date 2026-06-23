package changeloggen

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/gitexec"
	"github.com/mickeyyaya/evolveloop/go/test/fixtures"
)

// White-box, fast-tier coverage of the gitexec-backed cores behind ReadGitLog
// and VerifyRef. The raw exec.Command form could only be exercised against a
// real repo (git_integration_test.go); these pin the exact git invocation and
// the parse/error semantics via fixtures.FakeExec.

func TestReadGitLogWith_ParsesCommits(t *testing.T) {
	t.Parallel()
	const sep = "\x1f"
	fake := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git log": {Stdout: "sha1" + sep + "feat: one\nsha2" + sep + "fix: two\n"},
	}}
	g := gitexec.Git{Dir: "/repo", Exec: fake.Run}

	commits, err := ReadGitLogWith(context.Background(), g, "v0.1.0", "HEAD")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	want := []Commit{
		{SHA: "sha1", Subject: "feat: one"},
		{SHA: "sha2", Subject: "fix: two"},
	}
	if !reflect.DeepEqual(commits, want) {
		t.Errorf("commits = %+v, want %+v", commits, want)
	}
	if keys := fake.CallKeys(); !reflect.DeepEqual(keys, []string{"git log"}) {
		t.Fatalf("calls = %v, want [git log]", keys)
	}
	wantArgs := []string{"log", "--pretty=format:%H" + sep + "%s", "v0.1.0..HEAD"}
	if got := fake.Calls[0].Args; !reflect.DeepEqual(got, wantArgs) {
		t.Errorf("args = %v, want %v", got, wantArgs)
	}
}

func TestReadGitLogWith_EmptyRange_ReturnsErrNoCommits(t *testing.T) {
	t.Parallel()
	fake := &fixtures.FakeExec{} // empty stdout, exit 0
	_, err := ReadGitLogWith(context.Background(), gitexec.Git{Exec: fake.Run}, "HEAD", "HEAD")
	if !errors.Is(err, ErrNoCommits) {
		t.Errorf("err = %v, want ErrNoCommits", err)
	}
}

func TestReadGitLogWith_GitError_Wrapped(t *testing.T) {
	t.Parallel()
	fake := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git log": {ExitCode: 1},
	}}
	_, err := ReadGitLogWith(context.Background(), gitexec.Git{Exec: fake.Run}, "bad", "HEAD")
	if err == nil || errors.Is(err, ErrNoCommits) {
		t.Errorf("err = %v, want a non-nil git error (not ErrNoCommits)", err)
	}
}

func TestVerifyRefWith_ValidRef_OK(t *testing.T) {
	t.Parallel()
	fake := &fixtures.FakeExec{} // rev-parse succeeds (zero value)
	if err := VerifyRefWith(context.Background(), gitexec.Git{Dir: "/repo", Exec: fake.Run}, "v0.1.0"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if keys := fake.CallKeys(); !reflect.DeepEqual(keys, []string{"git rev-parse"}) {
		t.Fatalf("calls = %v, want [git rev-parse]", keys)
	}
	wantArgs := []string{"rev-parse", "--verify", "v0.1.0"}
	if got := fake.Calls[0].Args; !reflect.DeepEqual(got, wantArgs) {
		t.Errorf("args = %v, want %v", got, wantArgs)
	}
}

func TestVerifyRefWith_InvalidRef_Error(t *testing.T) {
	t.Parallel()
	fake := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git rev-parse": {ExitCode: 1},
	}}
	err := VerifyRefWith(context.Background(), gitexec.Git{Exec: fake.Run}, "nope")
	if err == nil {
		t.Fatalf("err = nil, want ref-does-not-exist error")
	}
}
