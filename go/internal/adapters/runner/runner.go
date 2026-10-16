// Package runner provides the two core.PhaseRunner adapters required by plan
// §2 "Approach C".
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
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type CmdRunner func(ctx context.Context, name string, args []string,
	stdin io.Reader, stdout, stderr io.Writer) (exitCode int, err error)

type SubprocessRunner struct {
	phaseName string
	binary    string
	runner    CmdRunner
}

func NewSubprocess(phaseName, binary string, runner CmdRunner) *SubprocessRunner {
	if runner == nil {
		runner = execRunner
	}
	return &SubprocessRunner{phaseName: phaseName, binary: binary, runner: runner}
}

func (s *SubprocessRunner) Name() string { return s.phaseName }

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

func PerPhase(phaseName string, inProc core.PhaseRunner, cmd CmdRunner) core.PhaseRunner {
	envKey := "EVOLVE_PHASE_" + strings.ToUpper(phaseName) + "_BIN"
	if binary := os.Getenv(envKey); binary != "" {
		return NewSubprocess(phaseName, binary, cmd)
	}
	return inProc
}

func execRunner(ctx context.Context, name string, args []string,
	stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	cmd := sysexec.Command(ctx, name, args...)
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
