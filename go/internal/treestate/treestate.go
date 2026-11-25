// Package treestate computes the tree-state SHA — sha256(`git diff HEAD`)
// — that binds a reviewed/audited change set to the exact tree being
// committed.
package treestate

import (
	"context"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func SHA(ctx context.Context, run sysexec.RunFunc, dir string, env []string) (string, error) {
	return diffSHA(ctx, run, dir, env, "HEAD")
}

func Between(ctx context.Context, run sysexec.RunFunc, dir string, env []string, from, to string) (string, error) {
	return diffSHA(ctx, run, dir, env, from, to)
}

func diffSHA(ctx context.Context, run sysexec.RunFunc, dir string, env []string, revs ...string) (string, error) {
	text, err := runDiff(ctx, run, dir, env, append([]string{"diff"}, revs...), RunError{Revs: revs})
	if err != nil {
		return "", err
	}
	return digest(text), nil
}

func runDiff(ctx context.Context, run sysexec.RunFunc, dir string, env, args []string, failure RunError) (string, error) {
	var buf strings.Builder
	var msg strings.Builder
	exitCode, err := run(ctx, "git", dir, args, env, nil, &buf, &msg)
	if err == nil && exitCode <= 1 {
		return buf.String(), nil
	}
	failure.ExitCode, failure.Err = exitCode, err
	if err == nil {
		failure.Stderr = strings.TrimSpace(msg.String())
	}
	return "", &failure
}

type RunError struct {
	ExitCode int
	Err      error
	Stderr   string
	Revs     []string
	Args     []string
}

func (e *RunError) command() string {
	if len(e.Args) > 0 {
		return "git " + strings.Join(e.Args, " ")
	}
	if len(e.Revs) == 0 {
		return "git diff HEAD"
	}
	return "git diff " + strings.Join(e.Revs, " ")
}

func (e *RunError) Error() string {
	if e.Err != nil {
		return e.command() + ": " + e.Err.Error()
	}
	msg := e.command() + " exit " + strconv.Itoa(e.ExitCode)
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

func (e *RunError) Unwrap() error { return e.Err }
