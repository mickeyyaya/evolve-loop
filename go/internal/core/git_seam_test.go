package core

import (
	"context"
	"io"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// gitRec is a recording sysexec.RunFunc for core's white-box git tests. core
// CANNOT use test/fixtures.FakeExec — fixtures imports core, so importing it
// from package core is an import cycle (the integration buildleak test uses
// real git for the same reason). gitRec mirrors the sysexec contract: a
// non-zero process exit returns (code, nil), never an error.
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

// useFakeGit swaps the package gitRunner seam for the recorder, restoring it on
// cleanup. White-box (package core) so it can reach the unexported seam — the
// whole point of S4.5 is that core's git access is now injectable, which was
// impossible while these functions shelled out via a hardcoded exec.Command.
func useFakeGit(t *testing.T, r *gitRec) {
	t.Helper()
	orig := gitRunner
	gitRunner = sysexec.RunFunc(r.run)
	t.Cleanup(func() { gitRunner = orig })
}

// TestRecoverBuildLeak_UsesInjectedGit is the S4.5 anchor: recoverBuildLeak must
// reach git through the package gitRunner seam (fakeable in the fast test tier),
// not a hardcoded exec.Command. Before S4.5 this test could not be written —
// recoverBuildLeak called the package-level gitCapture which shelled out to the
// real git binary, so the only coverage (buildleak_recover_test.go) needed an
// `//go:build integration` real repo.
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

// TestGitCapture_RoutesThroughInjectedSeam pins the chokepoint: ~25 core git
// calls funnel through gitCapture, which must use the gitRunner seam and
// preserve its contract — UNTRIMMED stdout, non-zero exit reported via the code.
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

// TestGitCapture_NonzeroExitReportedViaCode is load-bearing for callers that
// branch on the exit code (e.g. `merge-base --is-ancestor` rc=1): a non-zero
// exit is (code, nil), never an error.
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

// TestDefaultGitHEAD_UsesInjectedSeam proves the HEAD probe is faked too and
// still trims its output.
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

// TestDefaultGitHEAD_GitFailureDegrades preserves the contract that a failed
// HEAD probe degrades to ("", nil) — cycle-outcome labels degrade, the cycle
// continues — rather than erroring.
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
