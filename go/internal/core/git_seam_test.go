package core

import (
	"context"
	"io"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// gitRec is a recording sysexec.RunFunc for core's white-box git tests; core
// cannot import test/fixtures.FakeExec (import cycle), so this mirrors the
// sysexec contract directly: a non-zero process exit returns (code, nil).
type gitRec struct {
	stdout, stderr string
	exit           int
	calls          []gitCall
}

type gitCall struct {
	name, dir string
	args      []string
}

func (r *gitRec) run(_ context.Context, name, dir string, args, _ []string, _ io.Reader, out, errw io.Writer) (int, error) {
	r.calls = append(r.calls, gitCall{name: name, dir: dir, args: append([]string(nil), args...)})
	if out != nil && r.stdout != "" {
		_, _ = out.Write([]byte(r.stdout))
	}
	if errw != nil && r.stderr != "" {
		_, _ = errw.Write([]byte(r.stderr))
	}
	return r.exit, nil
}

// useFakeGit is white-box (package core) so it can reach the unexported
// gitRunner seam, swapping it for the recorder and restoring it on cleanup.
func useFakeGit(t *testing.T, r *gitRec) {
	t.Helper()
	orig := gitRunner
	gitRunner = sysexec.RunFunc(r.run)
	t.Cleanup(func() { gitRunner = orig })
}

func TestRecoverBuildLeak_UsesInjectedGit(t *testing.T) {
	r := &gitRec{stdout: ""} // empty porcelain → no leaks → clean true
	useFakeGit(t, r)

	if ok := recoverBuildLeak(context.Background(), "/proj", "/proj/.evolve/worktrees/cycle-1", map[string]bool{}, true); !ok {
		t.Fatalf("recoverBuildLeak = false, want true on a clean (no-leak) tree")
	}
	if len(r.calls) == 0 {
		t.Fatal("recoverBuildLeak issued no git calls through the injected seam — still a hardcoded exec.Command?")
	}
	c := r.calls[0]
	if c.name != "git" || c.dir != "/proj" {
		t.Errorf("first git call = {name:%q dir:%q}, want a git invocation rooted at /proj", c.name, c.dir)
	}
	if want := []string{"status", "--porcelain", "-uall"}; !reflect.DeepEqual(c.args, want) {
		t.Errorf("first git args = %v, want %v", c.args, want)
	}
}

func TestGitCapture_RoutesThroughInjectedSeam(t *testing.T) {
	r := &gitRec{stdout: "abc123\n"}
	useFakeGit(t, r)

	out, code, err := gitCapture(context.Background(), "/wt", "rev-parse", "HEAD")
	if err != nil || code != 0 {
		t.Fatalf("gitCapture = (code=%d, err=%v), want (0, nil)", code, err)
	}
	if out != "abc123\n" {
		t.Errorf("stdout = %q, want untrimmed %q (gitCapture must not trim)", out, "abc123\n")
	}
	if len(r.calls) != 1 || r.calls[0].dir != "/wt" {
		t.Errorf("recorded calls = %+v, want one git call in /wt", r.calls)
	}
}

func TestGitCapture_NonzeroExitReportedViaCode(t *testing.T) {
	r := &gitRec{exit: 1}
	useFakeGit(t, r)

	_, code, err := gitCapture(context.Background(), "/wt", "merge-base", "--is-ancestor", "x", "y")
	if err != nil {
		t.Fatalf("gitCapture err = %v, want nil (non-zero exit is not an error)", err)
	}
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
}

func TestDefaultGitHEAD_UsesInjectedSeam(t *testing.T) {
	r := &gitRec{stdout: "  deadbeef\n"}
	useFakeGit(t, r)

	head, err := defaultGitHEAD()
	if err != nil {
		t.Fatalf("defaultGitHEAD err = %v, want nil", err)
	}
	if head != "deadbeef" {
		t.Errorf("HEAD = %q, want trimmed %q", head, "deadbeef")
	}
}

func TestDefaultGitHEAD_GitFailureDegrades(t *testing.T) {
	r := &gitRec{exit: 128, stderr: "fatal: not a git repository"}
	useFakeGit(t, r)

	head, err := defaultGitHEAD()
	if err != nil {
		t.Fatalf("defaultGitHEAD err = %v, want nil (failure degrades, not errors)", err)
	}
	if head != "" {
		t.Errorf("HEAD = %q, want empty on git failure", head)
	}
}
