package subagent

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/subagent/subagentrun"
)

// contract.go is the host's view of the one verification ladder: "is this
// dispatched-agent artifact valid?" lives in internal/subagent/subagentrun
// (ADR-0103 unit 16) and every dispatch path — the run path, the Runner
// twin's classify, the fan-out parent's per-worker verifier — reaches it
// through these aliases and facades, so the three copies that once drifted
// apart cannot come back. (The lighter `evolve subagent check-token` probe in
// checktoken.go is intentionally a separate exists+token contract.)

// VerifyInput is the gathered evidence a dispatched agent's artifact is judged
// against — the leaf's type, verbatim.
type VerifyInput = subagentrun.VerifyInput

// VerifyResult is the verdict plus the integrity diagnostics that explain it
// (and, since unit 16, the typed rung).
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
