// Package treestate computes the tree-state SHA — sha256(`git diff HEAD`) —
// that binds a reviewed/audited change set to the exact tree being committed.
//
// It is the single source of truth for that fingerprint: the commit-gate
// attestation reader and the audit-binding verifier (both in
// internal/phases/ship) call [SHA] so writer and reader hash byte-identically.
// The git dependency is isolated behind a sysexec.RunFunc seam so the
// computation stays testable without a real repo.
//
// Exit-code semantics are load-bearing and preserved verbatim from the bash
// Auditor: `git diff HEAD` returns rc=1 when differences exist — that is NOT an
// error, only rc>1 (e.g. 128) is fatal. [SHA] therefore hashes the diff for any
// exit in {0,1} and surfaces rc>1 (and unrecoverable runner errors) as typed
// failures the caller can map to its own error vocabulary.
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

// SHA computes sha256(`git diff HEAD`) and returns it as a lowercase hex string.
//
// run is the command-execution seam (production callers pass
// sysexec.DefaultRunner; tests inject a fake). dir is the git working directory;
// "" inherits the caller's cwd, matching sysexec.RunFunc semantics. env is the
// process environment passed straight through to run (nil inherits the parent).
//
// `git diff HEAD` exit 1 (differences present) is normal and is hashed like exit
// 0; exit >1 is fatal and returns a *RunError with ExitCode set and Err nil. An
// unrecoverable runner failure (binary missing, context cancelled) returns a
// *RunError with Err set and ExitCode -1. On success Err is nil and the returned
// string is the 64-char hex digest.
func SHA(ctx context.Context, run sysexec.RunFunc, dir string, env []string) (string, error) {
	var buf strings.Builder
	exitCode, err := run(ctx, "git", dir, []string{"diff", "HEAD"}, env, nil, &buf, io.Discard)
	if err != nil {
		return "", &RunError{ExitCode: exitCode, Err: err}
	}
	if exitCode > 1 {
		// rc=1 from git diff is normal (differences). rc=128 is fatal.
		return "", &RunError{ExitCode: exitCode}
	}
	h := sha256.New()
	_, _ = h.Write([]byte(buf.String()))
	return hex.EncodeToString(h.Sum(nil)), nil
}

// RunError reports a failed `git diff HEAD` invocation behind [SHA]. Exactly one
// of two shapes occurs: Err != nil (unrecoverable runner failure, ExitCode -1)
// or Err == nil with ExitCode > 1 (a fatal git exit such as 128). Callers map it
// to their own error codes; the two shapes are distinguishable via Err.
type RunError struct {
	// ExitCode is the process exit code (-1 for an unrecoverable runner error).
	ExitCode int
	// Err is the underlying runner error, or nil for a fatal git exit (>1).
	Err error
}

// Error implements error.
func (e *RunError) Error() string {
	if e.Err != nil {
		return "git diff HEAD: " + e.Err.Error()
	}
	return "git diff HEAD exit " + strconv.Itoa(e.ExitCode)
}

// Unwrap exposes the underlying runner error to errors.Is/As; nil for a fatal
// git exit.
func (e *RunError) Unwrap() error { return e.Err }
