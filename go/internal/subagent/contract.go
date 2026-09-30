package subagent

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/subagent/subagentrun"
)

// VerifyInput is the gathered evidence a dispatched agent's artifact is judged
// against.
type VerifyInput = subagentrun.VerifyInput

// VerifyResult is the verdict plus the integrity diagnostics that explain it.
type VerifyResult = subagentrun.VerifyResult

// Verify applies the one ordered verdict ladder — integrity first, exec last.
func Verify(in VerifyInput) VerifyResult { return subagentrun.Verify(in) }

// VerifyArtifact is the one I/O-bearing entry point every dispatch path shares.
func VerifyArtifact(
	stat func(path string) (time.Time, error),
	read func(path string) ([]byte, error),
	now func() time.Time,
	artifactPath, token string,
	exitCode int,
	execErr error,
) VerifyResult {
	return subagentrun.VerifyArtifact(stat, read, now, artifactPath, token, exitCode, execErr)
}
