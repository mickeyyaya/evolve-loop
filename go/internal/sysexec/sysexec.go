package sysexec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type RunFunc func(ctx context.Context, name, dir string, args, env []string,
	stdin io.Reader, stdout, stderr io.Writer) (exitCode int, err error)

func DefaultRunner(ctx context.Context, name, dir string, args, env []string,
	stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	cmd := Command(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success() {
			fmt.Fprintf(os.Stderr, "sysexec: WARN %s: a child process kept the output open for %s after the exit; the output can be incomplete\n", name, WaitDelay)
			return 0, nil
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}

func Capture(ctx context.Context, run RunFunc, dir, name string, args ...string) (stdout, stderr string, exitCode int, err error) {
	var o, e strings.Builder
	code, runErr := run(ctx, name, dir, args, nil, nil, &o, &e)
	return o.String(), e.String(), code, runErr
}

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

func CombinedOutput(ctx context.Context, run RunFunc, dir, name, stdin string, args ...string) (string, error) {
	var buf strings.Builder
	var in io.Reader
	if stdin != "" {
		in = strings.NewReader(stdin)
	}
	_, err := run(ctx, name, dir, args, nil, in, &buf, &buf)
	return buf.String(), err
}
