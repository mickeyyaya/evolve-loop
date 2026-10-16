package phaseproto

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	CodeHandlerError = "HANDLER_ERROR"
	CodeChildCrashed = "CHILD_CRASHED"
	CodeDecodeFailed = "DECODE_FAILED"
)

var correlationIDFn = defaultCorrelationID

type SubprocessRunner struct {
	name string
	bin  string
	args []string
	env  []string
}

func NewSubprocessRunner(name, bin string, args, extraEnv []string) *SubprocessRunner {
	return &SubprocessRunner{name: name, bin: bin, args: args, env: extraEnv}
}

func (r *SubprocessRunner) Name() string { return r.name }

func (r *SubprocessRunner) buildCmd(ctx context.Context) *exec.Cmd {
	cmd := sysexec.Command(ctx, r.bin, r.args...)
	cmd.Env = append(os.Environ(), r.env...)
	return cmd
}

func (r *SubprocessRunner) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	cmd := r.buildCmd(ctx)

	corr := correlationIDFn()
	envIn, err := EncodeRequest(corr, req)
	if err != nil {
		return core.PhaseResponse{}, err
	}
	raw, err := json.Marshal(envIn)
	if err != nil {
		return core.PhaseResponse{}, fmt.Errorf("phaseproto: marshal request envelope: %w", err)
	}

	cmd.Stdin = bytes.NewReader(append(raw, '\n'))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if runErr := cmd.Run(); runErr != nil {
		return core.PhaseResponse{}, &WireError{
			Code:      CodeChildCrashed,
			Message:   fmt.Sprintf("subprocess exited with error: %v; stderr=%q", runErr, stderr.String()),
			Retryable: true,
		}
	}

	line, err := readOneEnvelopeLine(&stdout)
	if err != nil {
		return core.PhaseResponse{}, &WireError{
			Code:      CodeDecodeFailed,
			Message:   fmt.Sprintf("read response: %v; stderr=%q", err, stderr.String()),
			Retryable: false,
		}
	}
	var envOut Envelope
	if err := json.Unmarshal(line, &envOut); err != nil {
		return core.PhaseResponse{}, &WireError{
			Code:      CodeDecodeFailed,
			Message:   fmt.Sprintf("unmarshal response: %v", err),
			Retryable: false,
		}
	}
	if envOut.Kind == KindError {
		if envOut.Error == nil {
			return core.PhaseResponse{}, &WireError{Code: CodeDecodeFailed, Message: "error envelope missing error field"}
		}
		return core.PhaseResponse{}, envOut.Error
	}
	resp, err := DecodeResponse(envOut)
	if err != nil {
		return core.PhaseResponse{}, &WireError{Code: CodeDecodeFailed, Message: err.Error()}
	}
	return resp, nil
}

func readOneEnvelopeLine(r *bytes.Buffer) ([]byte, error) {
	br := bufio.NewReader(r)
	line, err := br.ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return nil, err
	}
	line = bytes.TrimRight(line, "\n")
	if len(line) == 0 {
		return nil, fmt.Errorf("empty stdout")
	}
	return line, nil
}

func defaultCorrelationID() string {
	return fmt.Sprintf("pp-%d", os.Getpid())
}
