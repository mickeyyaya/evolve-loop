// Package runner provides the two core.PhaseRunner adapters required
// by plan §2 "Approach C":
//
//  1. The phase's own in-process Go impl is itself a PhaseRunner; the
//     orchestrator just calls it directly. No wrapper needed.
//
//  2. SubprocessRunner forks an external binary and speaks a JSON-on-
//     stdin / JSON-on-stdout protocol so third parties can implement
//     phases in any language.
//
// The PerPhase factory consults EVOLVE_PHASE_<NAME>_BIN at runtime and
// returns whichever runner is configured for that phase. The
// orchestrator sees only core.PhaseRunner — the independence guarantee.
package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// CmdRunner is the seam for injecting subprocess behavior in tests.
type CmdRunner func(ctx context.Context, name string, args []string,
	stdin io.Reader, stdout, stderr io.Writer) (exitCode int, err error)

// SubprocessRunner implements core.PhaseRunner by forking a binary
// per call. Each Run() writes the PhaseRequest as JSON to the
// subprocess stdin, reads stdout as the PhaseResponse JSON, and
// captures stderr for diagnostics on non-zero exit.
type SubprocessRunner struct {
	phaseName string
	binary    string
	runner    CmdRunner
}

// NewSubprocess constructs a SubprocessRunner. Pass nil runner to use
// the default exec.CommandContext-based runner (production wiring).
func NewSubprocess(phaseName, binary string, runner CmdRunner) *SubprocessRunner {
	if runner == nil {
		runner = execRunner
	}
	return &SubprocessRunner{phaseName: phaseName, binary: binary, runner: runner}
}

// Name returns the phase name this runner is bound to.
func (s *SubprocessRunner) Name() string { return s.phaseName }

// Run executes the subprocess for one PhaseRequest.
func (s *SubprocessRunner) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return core.PhaseResponse{}, fmt.Errorf("runner: marshal request: %w", err)
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	exitCode, runErr := s.runner(ctx, s.binary, nil, bytes.NewReader(reqBytes), &stdoutBuf, &stderrBuf)
	if runErr != nil {
		return core.PhaseResponse{}, fmt.Errorf("runner: exec %s: %w", s.binary, runErr)
	}
	if exitCode != 0 {
		stderrTail := stderrBuf.String()
		if len(stderrTail) > 400 {
			stderrTail = stderrTail[:400] + "…"
		}
		return core.PhaseResponse{}, fmt.Errorf("runner: %s exit=%d: %s", s.phaseName, exitCode, stderrTail)
	}
	var resp core.PhaseResponse
	if err := json.Unmarshal(stdoutBuf.Bytes(), &resp); err != nil {
		return core.PhaseResponse{}, fmt.Errorf("runner: parse response from %s: %w", s.phaseName, err)
	}
	return resp, nil
}

// PerPhase picks the runner for a given phase name based on
// EVOLVE_PHASE_<NAME>_BIN: when set, returns a SubprocessRunner
// wrapping that binary; otherwise returns inProc unchanged.
//
// The phase name is upper-cased to match the CLAUDE.md env-var
// convention (EVOLVE_PHASE_BUILD_BIN, not EVOLVE_PHASE_build_BIN).
func PerPhase(phaseName string, inProc core.PhaseRunner, cmd CmdRunner) core.PhaseRunner {
	envKey := "EVOLVE_PHASE_" + strings.ToUpper(phaseName) + "_BIN"
	if binary := os.Getenv(envKey); binary != "" {
		return NewSubprocess(phaseName, binary, cmd)
	}
	return inProc
}

// execRunner is the production CmdRunner. Maps exec.ExitError to
// exitCode + nil err so the adapter's exit-code-based branching works.
func execRunner(ctx context.Context, name string, args []string,
	stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
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
