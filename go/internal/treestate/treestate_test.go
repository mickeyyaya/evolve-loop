// treestate_test.go — golden + parity coverage for SHA.
//
// Two layers: (1) a deterministic table over the injected sysexec.RunFunc seam
// (fixed git-diff bytes -> independently-computed sha256, plus the exit-code and
// runner-error failure shapes), and (2) a real-git parity check that hashes
// `git diff HEAD` in a temp repo and cross-validates against the shell
// `git diff HEAD | shasum -a 256` the bash Auditor uses — proving the Go and
// shell fingerprints are byte-identical. Tests are -race-safe (no shared state).
package treestate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// fakeRunner returns a sysexec.RunFunc that writes diffOut to stdout and returns
// (exit, runErr), capturing the args it was called with for assertion.
func fakeRunner(diffOut string, exit int, runErr error, gotArgs *[]string, gotDir *string) sysexec.RunFunc {
	return func(_ context.Context, name, dir string, args, _ []string,
		_ io.Reader, stdout, _ io.Writer) (int, error) {
		if gotArgs != nil {
			*gotArgs = append([]string{name}, args...)
		}
		if gotDir != nil {
			*gotDir = dir
		}
		if stdout != nil {
			_, _ = io.WriteString(stdout, diffOut)
		}
		return exit, runErr
	}
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestSHA_TableSeam(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	cases := []struct {
		name     string
		diff     string
		exit     int
		runErr   error
		wantSHA  string
		wantErr  bool
		wantExit int   // expected RunError.ExitCode when wantErr
		wantWrap error // expected RunError.Err (nil for fatal-exit shape)
	}{
		{
			// Empty diff (clean tree) hashes the empty string.
			name:    "clean-tree-exit0",
			diff:    "",
			exit:    0,
			wantSHA: sha256Hex(""),
		},
		{
			// rc=1 means "differences present" — NOT an error; the diff is hashed.
			name:    "differences-exit1-hashed",
			diff:    "diff --git a/x b/x\n",
			exit:    1,
			wantSHA: sha256Hex("diff --git a/x b/x\n"),
		},
		{
			// rc=128 (fatal git error) — RunError with nil Err.
			name:     "fatal-exit128",
			diff:     "ignored",
			exit:     128,
			wantErr:  true,
			wantExit: 128,
			wantWrap: nil,
		},
		{
			// Unrecoverable runner failure (binary missing etc.) — RunError wraps Err.
			name:     "runner-error",
			diff:     "",
			exit:     -1,
			runErr:   boom,
			wantErr:  true,
			wantExit: -1,
			wantWrap: boom,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var gotArgs []string
			var gotDir string
			run := fakeRunner(tc.diff, tc.exit, tc.runErr, &gotArgs, &gotDir)
			got, err := SHA(context.Background(), run, "/some/dir", nil)

			// The seam must always be invoked as `git diff HEAD` in the given dir.
			wantArgs := []string{"git", "diff", "HEAD"}
			if strings.Join(gotArgs, " ") != strings.Join(wantArgs, " ") {
				t.Fatalf("git invocation = %v, want %v", gotArgs, wantArgs)
			}
			if gotDir != "/some/dir" {
				t.Fatalf("dir = %q, want /some/dir", gotDir)
			}

			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got sha %q", got)
				}
				var re *RunError
				if !errors.As(err, &re) {
					t.Fatalf("want *RunError, got %T (%v)", err, err)
				}
				if re.ExitCode != tc.wantExit {
					t.Errorf("RunError.ExitCode = %d, want %d", re.ExitCode, tc.wantExit)
				}
				if tc.wantWrap != nil && !errors.Is(err, tc.wantWrap) {
					t.Errorf("errors.Is(err, wantWrap) = false; err = %v", err)
				}
				if tc.wantWrap == nil && re.Err != nil {
					t.Errorf("RunError.Err = %v, want nil for fatal-exit shape", re.Err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantSHA {
				t.Errorf("SHA = %q, want %q", got, tc.wantSHA)
			}
		})
	}
}

// TestRunError_MessagesAndUnwrap names and exercises RunError.Error and
// RunError.Unwrap directly, locking the exact message strings the ship wrapper
// re-derives so the post-extraction error text stays byte-identical.
func TestRunError_MessagesAndUnwrap(t *testing.T) {
	t.Parallel()

	// Fatal-exit shape: Err nil, message carries the code, Unwrap is nil.
	fatal := &RunError{ExitCode: 128}
	if got, want := fatal.Error(), "git diff HEAD exit 128"; got != want {
		t.Errorf("fatal.Error() = %q, want %q", got, want)
	}
	if fatal.Unwrap() != nil {
		t.Errorf("fatal.Unwrap() = %v, want nil", fatal.Unwrap())
	}

	// Runner-error shape: Err wrapped, message includes it, Unwrap returns it.
	boom := errors.New("boom")
	runErr := &RunError{ExitCode: -1, Err: boom}
	if got, want := runErr.Error(), "git diff HEAD: boom"; got != want {
		t.Errorf("runErr.Error() = %q, want %q", got, want)
	}
	if runErr.Unwrap() != boom {
		t.Errorf("runErr.Unwrap() = %v, want boom", runErr.Unwrap())
	}
	if !errors.Is(runErr, boom) {
		t.Error("errors.Is(runErr, boom) = false, want true")
	}
}

// TestSHA_RealGitParity proves the Go fingerprint equals the shell pipeline
// `git diff HEAD | shasum -a 256` over a real working tree — the exact invariant
// the commit-gate/audit binding depends on.
func TestSHA_RealGitParity(t *testing.T) {
	t.Parallel()
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	shaTool, shaArgs := lookHasher(t)

	dir := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command(gitBin, args...)
		cmd.Dir = dir
		// Deterministic identity so the diff (and thus the SHA) is reproducible.
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "f.txt")
	runGit("commit", "-q", "-m", "init")
	// Mutate a tracked file so `git diff HEAD` is non-empty.
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Go path.
	goSHA, err := SHA(context.Background(), sysexec.DefaultRunner, dir, os.Environ())
	if err != nil {
		t.Fatalf("treestate.SHA: %v", err)
	}

	// Shell path: `git diff HEAD` piped through the system hasher.
	diffCmd := exec.Command(gitBin, "diff", "HEAD")
	diffCmd.Dir = dir
	diffOut, err := diffCmd.Output()
	if err != nil {
		t.Fatalf("git diff HEAD: %v", err)
	}
	if len(diffOut) == 0 {
		t.Fatal("expected a non-empty diff for the parity check")
	}
	shellSHA := hashViaShell(t, shaTool, shaArgs, diffOut)

	if goSHA != shellSHA {
		t.Fatalf("parity mismatch: go=%s shell=%s", goSHA, shellSHA)
	}
}

// lookHasher finds shasum (-a 256) or sha256sum, skipping if neither exists.
func lookHasher(t *testing.T) (string, []string) {
	t.Helper()
	if p, err := exec.LookPath("shasum"); err == nil {
		return p, []string{"-a", "256"}
	}
	if p, err := exec.LookPath("sha256sum"); err == nil {
		return p, nil
	}
	t.Skip("no shasum/sha256sum on PATH")
	return "", nil
}

// hashViaShell runs the system sha256 tool over data and returns the lowercase
// hex digest (the leading field of the tool's output).
func hashViaShell(t *testing.T, tool string, args []string, data []byte) string {
	t.Helper()
	cmd := exec.Command(tool, args...)
	cmd.Stdin = strings.NewReader(string(data))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		t.Fatalf("%s produced no output", tool)
	}
	return fields[0]
}
