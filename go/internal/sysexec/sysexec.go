package sysexec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// RunFunc is THE command-execution seam. It is the superset every legacy
// runner signature reduces to: a working directory, an environment, piped
// stdin, streamed stdout/stderr, a process exit code, and a Go error.
//
// Contract:
//   - exitCode is the process exit status. A process that runs and exits
//     non-zero returns (code, nil) — NOT an error. This is load-bearing:
//     callers branch on exitCode (e.g. `git diff --quiet` rc=1 means
//     "differences", not "failure").
//   - err is non-nil ONLY for unrecoverable failures: binary-not-found,
//     context cancellation, or pipe setup. In that case exitCode is -1.
//   - dir == "" inherits the caller's working directory.
//   - env == nil inherits the parent environment (os.Environ()); a non-nil env
//     REPLACES the process environment (standard os/exec semantics).
//   - stdin == nil means no stdin; stdout/stderr == nil discard that stream.
type RunFunc func(ctx context.Context, name, dir string, args, env []string,
	stdin io.Reader, stdout, stderr io.Writer) (exitCode int, err error)

// DefaultRunner is the production [RunFunc]. It wraps exec.CommandContext and
// honors the RunFunc contract: a non-zero process exit maps to (code, nil);
// err is reserved for unrecoverable failures and pairs with exitCode -1.
func DefaultRunner(ctx context.Context, name, dir string, args, env []string,
	stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir // "" => inherit caller cwd
	cmd.Env = env // nil => inherit os.Environ()
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}

// Capture runs name+args in dir via run and returns stdout and stderr as
// strings alongside the exit code. err is non-nil only for unrecoverable
// failures (see [RunFunc]); a non-zero exit is reported via exitCode, leaving
// the caller to decide whether that constitutes a failure.
func Capture(ctx context.Context, run RunFunc, dir, name string, args ...string) (stdout, stderr string, exitCode int, err error) {
	var o, e strings.Builder
	code, runErr := run(ctx, name, dir, args, nil, nil, &o, &e)
	return o.String(), e.String(), code, runErr
}

// Output is the ergonomic command-query helper: it returns trimmed stdout and
// treats ANY non-zero exit (or unrecoverable error) as an error, folding the
// exit code and stderr into the message. Use it for queries where a non-zero
// exit IS a failure (git rev-parse, describe, symbolic-ref). Callers that must
// inspect a specific non-zero code (e.g. `git diff --quiet` rc=1) use [Capture].
func Output(ctx context.Context, run RunFunc, dir, name string, args ...string) (string, error) {
	out, errOut, code, err := Capture(ctx, run, dir, name, args...)
	if err != nil {
		return "", fmt.Errorf("sysexec: %s %v: %w", name, args, err)
	}
	if code != 0 {
		return "", fmt.Errorf("sysexec: %s %v exit=%d: %s", name, args, code, strings.TrimSpace(errOut))
	}
	return strings.TrimSpace(out), nil
}

// CombinedOutput mirrors exec.Cmd.CombinedOutput: stdout and stderr interleave
// into one buffer. Some CLI classifiers frame their reply with header/footer
// lines on an unspecified stream, so callers need the merged transcript. An
// empty stdin is passed as no stdin. The returned error is whatever run
// returns (unrecoverable only); a non-zero exit is not folded in here because
// combined-output callers parse the buffer regardless of exit status.
//
// Passing one *strings.Builder as both stdout and stderr is race-free on the
// DefaultRunner path: os/exec detects the shared writer (interfaceEqual) and
// reuses a single pipe + copy goroutine, exactly as the stdlib's own
// Cmd.CombinedOutput does. TestCombinedOutput_RealRunner_NoRace enforces this
// under -race.
func CombinedOutput(ctx context.Context, run RunFunc, dir, name, stdin string, args ...string) (string, error) {
	var buf strings.Builder
	var in io.Reader
	if stdin != "" {
		in = strings.NewReader(stdin)
	}
	_, err := run(ctx, name, dir, args, nil, in, &buf, &buf)
	return buf.String(), err
}
