// Package treestate computes the tree-state SHA — sha256(`git diff HEAD`)
// — that binds a reviewed/audited change set to the exact tree being
// committed.
package treestate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	var buf strings.Builder
	var msg strings.Builder
	exitCode, err := run(ctx, "git", dir, append([]string{"diff"}, revs...), env, nil, &buf, &msg)
	if err != nil {
		return "", &RunError{ExitCode: exitCode, Err: err, Revs: revs}
	}
	if exitCode > 1 {
		return "", &RunError{ExitCode: exitCode, Stderr: strings.TrimSpace(msg.String()), Revs: revs}
	}
	h := sha256.New()
	_, _ = h.Write([]byte(buf.String()))
	return hex.EncodeToString(h.Sum(nil)), nil
}

type RunError struct {
	ExitCode int
	Err      error
	Stderr   string
	Revs     []string
}

func (e *RunError) command() string {
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
