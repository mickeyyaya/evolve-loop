package sysexec_test

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/sysexec"
)

// DefaultRunner is exercised against real, ubiquitous shell commands — it is
// the one production wrapper that genuinely must fork to be meaningful. The
// forks are trivial (sh -c) so the test stays in the fast tier.

func TestDefaultRunner_ExitZero_NoError(t *testing.T) {
	code, err := sysexec.DefaultRunner(context.Background(), "sh", "", []string{"-c", "exit 0"}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if code != 0 {
		t.Fatalf("exitCode = %d, want 0", code)
	}
}

// The load-bearing contract: a non-zero exit is (code, nil), NOT an error.
// Callers branch on exit codes (git diff --quiet rc=1 == "differences").
func TestDefaultRunner_NonZeroExit_IsCodeNotError(t *testing.T) {
	code, err := sysexec.DefaultRunner(context.Background(), "sh", "", []string{"-c", "exit 3"}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("err = %v, want nil (non-zero exit must not be an error)", err)
	}
	if code != 3 {
		t.Fatalf("exitCode = %d, want 3", code)
	}
}

// An unrecoverable failure (binary not found) is err != nil with exitCode -1.
func TestDefaultRunner_BinaryNotFound_IsUnrecoverableError(t *testing.T) {
	code, err := sysexec.DefaultRunner(context.Background(), "evolve-no-such-binary-xyz", "", nil, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("err = nil, want non-nil for missing binary")
	}
	if code != -1 {
		t.Fatalf("exitCode = %d, want -1 for unrecoverable error", code)
	}
}

func TestDefaultRunner_CapturesStdoutAndStderr(t *testing.T) {
	var out, errBuf strings.Builder
	code, err := sysexec.DefaultRunner(context.Background(), "sh", "",
		[]string{"-c", "printf hello; printf oops 1>&2"}, nil, nil, &out, &errBuf)
	if err != nil || code != 0 {
		t.Fatalf("code=%d err=%v, want 0/nil", code, err)
	}
	if out.String() != "hello" {
		t.Fatalf("stdout = %q, want %q", out.String(), "hello")
	}
	if errBuf.String() != "oops" {
		t.Fatalf("stderr = %q, want %q", errBuf.String(), "oops")
	}
}

func TestDefaultRunner_RespectsWorkingDir(t *testing.T) {
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	var out strings.Builder
	code, runErr := sysexec.DefaultRunner(context.Background(), "sh", dir, []string{"-c", "pwd -P"}, nil, nil, &out, nil)
	if runErr != nil || code != 0 {
		t.Fatalf("code=%d err=%v, want 0/nil", code, runErr)
	}
	if got := strings.TrimSpace(out.String()); got != resolved {
		t.Fatalf("pwd = %q, want %q (dir was not honored)", got, resolved)
	}
}

func TestDefaultRunner_PipesStdin(t *testing.T) {
	var out strings.Builder
	code, err := sysexec.DefaultRunner(context.Background(), "cat", "", nil, nil, strings.NewReader("piped-in"), &out, nil)
	if err != nil || code != 0 {
		t.Fatalf("code=%d err=%v, want 0/nil", code, err)
	}
	if out.String() != "piped-in" {
		t.Fatalf("stdout = %q, want %q", out.String(), "piped-in")
	}
}

func TestDefaultRunner_ContextCancelled_ReturnsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before run
	code, err := sysexec.DefaultRunner(ctx, "sh", "", []string{"-c", "sleep 5"}, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("err = nil, want context-cancellation error")
	}
	if code != -1 {
		t.Fatalf("exitCode = %d, want -1", code)
	}
}

// --- Convenience helpers (driven via a tiny inline RunFunc stub so the helper
// logic is tested without forking and without depending on fixtures). ---

func stubRun(stdout, stderr string, code int, err error) sysexec.RunFunc {
	return func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, so, se io.Writer) (int, error) {
		if so != nil && stdout != "" {
			_, _ = so.Write([]byte(stdout))
		}
		if se != nil && stderr != "" {
			_, _ = se.Write([]byte(stderr))
		}
		return code, err
	}
}

func TestCapture_ReturnsStreamsAndCode(t *testing.T) {
	out, errOut, code, err := sysexec.Capture(context.Background(), stubRun("OUT", "ERR", 2, nil), "/wd", "git", "status")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if out != "OUT" || errOut != "ERR" || code != 2 {
		t.Fatalf("Capture = (%q,%q,%d), want (OUT,ERR,2)", out, errOut, code)
	}
}

func TestOutput_TrimsTrailingWhitespace(t *testing.T) {
	got, err := sysexec.Output(context.Background(), stubRun("abc123\n", "", 0, nil), "", "git", "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got != "abc123" {
		t.Fatalf("Output = %q, want %q (must trim)", got, "abc123")
	}
}

func TestOutput_NonZeroExit_IsError(t *testing.T) {
	_, err := sysexec.Output(context.Background(), stubRun("", "fatal: bad revision\n", 128, nil), "", "git", "rev-parse", "nope")
	if err == nil {
		t.Fatal("err = nil, want error for non-zero exit")
	}
	if !strings.Contains(err.Error(), "128") || !strings.Contains(err.Error(), "bad revision") {
		t.Fatalf("err = %v, want it to mention exit code and stderr", err)
	}
}

func TestCombinedOutput_InterleavesStdoutStderr(t *testing.T) {
	got, err := sysexec.CombinedOutput(context.Background(), stubRun("out", "err", 0, nil), "", "codex", "")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	// Both streams land in one buffer (order is stub-dependent: stdout then stderr).
	if !strings.Contains(got, "out") || !strings.Contains(got, "err") {
		t.Fatalf("CombinedOutput = %q, want it to contain both out and err", got)
	}
}

// Proof (under -race) that CombinedOutput is race-free on the REAL DefaultRunner
// path despite passing one buffer as both stdout and stderr: os/exec reuses a
// single pipe + goroutine for a shared writer, so the two streams never write
// concurrently. A real fork is intentional here — it is the only way to exercise
// the os/exec sharing behavior the helper relies on.
func TestCombinedOutput_RealRunner_NoRace(t *testing.T) {
	got, err := sysexec.CombinedOutput(context.Background(), sysexec.DefaultRunner, "", "sh", "",
		"-c", "printf out; printf err 1>&2")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !strings.Contains(got, "out") || !strings.Contains(got, "err") {
		t.Fatalf("combined = %q, want both out and err", got)
	}
}
