// Package treestate computes the tree-state SHA — sha256(`git diff HEAD`)
// — that binds a reviewed/audited change set to the exact tree being
// committed.
package treestate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func SHA(ctx context.Context, run sysexec.RunFunc, dir string, env []string) (string, error) {
	var buf strings.Builder
	exitCode, err := run(ctx, "git", dir, []string{"diff", "HEAD"}, env, nil, &buf, io.Discard)
	if err != nil {
		return "", &RunError{ExitCode: exitCode, Err: err}
	}
	if exitCode > 1 {
		return "", &RunError{ExitCode: exitCode}
	}
	h := sha256.New()
	_, _ = h.Write([]byte(buf.String()))
	return hex.EncodeToString(h.Sum(nil)), nil
}

type RunError struct {
	ExitCode int
	Err      error
}

func (e *RunError) Error() string {
	if e.Err != nil {
		return "git diff HEAD: " + e.Err.Error()
	}
	return "git diff HEAD exit " + strconv.Itoa(e.ExitCode)
}

func (e *RunError) Unwrap() error { return e.Err }
