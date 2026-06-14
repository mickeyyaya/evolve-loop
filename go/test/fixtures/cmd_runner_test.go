package fixtures_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestFakeExec_ZeroValue_SucceedsWithEmptyOutput(t *testing.T) {
	t.Parallel()
	f := &fixtures.FakeExec{}
	var out strings.Builder
	code, err := f.Run(context.Background(), "git", "", []string{"status"}, nil, nil, &out, nil)
	if err != nil || code != 0 {
		t.Fatalf("zero-value FakeExec: code=%d err=%v, want 0/nil", code, err)
	}
	if out.String() != "" {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
}

func TestFakeExec_ScriptsBySubcommand(t *testing.T) {
	t.Parallel()
	f := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git rev-parse": {Stdout: "abc123\n"},
	}}
	got, err := sysexec.Output(context.Background(), f.Run, "", "git", "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("Output via FakeExec: %v", err)
	}
	if got != "abc123" {
		t.Fatalf("Output = %q, want abc123 (Run must be a valid sysexec.RunFunc)", got)
	}
}

func TestFakeExec_KeySkipsLeadingFlags(t *testing.T) {
	t.Parallel()
	f := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git status": {Stdout: "clean"},
	}}
	var out strings.Builder
	// `git -C /wt -c k=v status` must resolve to the "git status" key.
	_, err := f.Run(context.Background(), "git", "", []string{"-C", "/wt", "-c", "k=v", "status"}, nil, nil, &out, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.String() != "clean" {
		t.Fatalf("stdout = %q, want clean (flag-skipping key derivation failed)", out.String())
	}
}

func TestFakeExec_RecordsCallsInOrderWithContext(t *testing.T) {
	t.Parallel()
	f := &fixtures.FakeExec{}
	ctx := context.Background()
	_, _ = f.Run(ctx, "git", "/wt", []string{"add", "-A"}, []string{"X=1"}, strings.NewReader("body"), nil, nil)
	_, _ = f.Run(ctx, "git", "/wt", []string{"commit", "-m", "x"}, nil, nil, nil, nil)

	if got := f.CallKeys(); len(got) != 2 || got[0] != "git add" || got[1] != "git commit" {
		t.Fatalf("CallKeys = %v, want [git add, git commit]", got)
	}
	c0 := f.Calls[0]
	if c0.Dir != "/wt" {
		t.Fatalf("Calls[0].Dir = %q, want /wt", c0.Dir)
	}
	if len(c0.Env) != 1 || c0.Env[0] != "X=1" {
		t.Fatalf("Calls[0].Env = %v, want [X=1]", c0.Env)
	}
	if c0.Stdin != "body" {
		t.Fatalf("Calls[0].Stdin = %q, want body (stdin must be captured)", c0.Stdin)
	}
}

func TestFakeExec_InjectsNonZeroExitWithoutError(t *testing.T) {
	t.Parallel()
	f := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"git diff": {ExitCode: 1}, // "differences" — a code, not an error
	}}
	code, err := f.Run(context.Background(), "git", "", []string{"diff", "--quiet"}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("err = %v, want nil (non-zero exit is not an error)", err)
	}
	if code != 1 {
		t.Fatalf("exitCode = %d, want 1", code)
	}
}

func TestFakeExec_InjectsUnrecoverableError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("binary not found")
	f := &fixtures.FakeExec{Scripts: map[string]fixtures.ExecResponse{
		"gh release": {Err: sentinel},
	}}
	code, err := f.Run(context.Background(), "gh", "", []string{"release", "view", "v1"}, nil, nil, nil, nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want sentinel", err)
	}
	if code != -1 {
		t.Fatalf("exitCode = %d, want -1 for unrecoverable error", code)
	}
}

func TestFakeExec_NameOnlyFallbackAndDefault(t *testing.T) {
	t.Parallel()
	f := &fixtures.FakeExec{
		Scripts: map[string]fixtures.ExecResponse{"tmux": {Stdout: "name-match"}},
		Default: fixtures.ExecResponse{Stdout: "default-match"},
	}
	var out1, out2 strings.Builder
	// No "tmux ls" key, but a name-only "tmux" key exists → name fallback.
	_, _ = f.Run(context.Background(), "tmux", "", []string{"ls"}, nil, nil, &out1, nil)
	if out1.String() != "name-match" {
		t.Fatalf("stdout = %q, want name-match (name-only fallback)", out1.String())
	}
	// Unknown binary → Default.
	_, _ = f.Run(context.Background(), "brew", "", []string{"list"}, nil, nil, &out2, nil)
	if out2.String() != "default-match" {
		t.Fatalf("stdout = %q, want default-match (Default fallback)", out2.String())
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("stdin boom") }

// A stdin reader that errors is a test-setup bug — FakeExec must surface it
// (fail loudly), not silently truncate the captured stdin.
func TestFakeExec_StdinReadError_FailsLoudly(t *testing.T) {
	t.Parallel()
	f := &fixtures.FakeExec{}
	code, err := f.Run(context.Background(), "gh", "", []string{"release", "create"}, nil, errReader{}, nil, nil)
	if err == nil {
		t.Fatal("err = nil, want the stdin read error surfaced")
	}
	if code != -1 {
		t.Fatalf("exitCode = %d, want -1", code)
	}
	if len(f.Calls) != 1 {
		t.Fatalf("Calls = %d, want 1 (call recorded even on stdin error)", len(f.Calls))
	}
}
