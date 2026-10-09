package subagent

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/subagent/subagentrun"
)

type VerifyInput = subagentrun.VerifyInput

type VerifyResult = subagentrun.VerifyResult

func Verify(in VerifyInput) VerifyResult { return subagentrun.Verify(in) }

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
