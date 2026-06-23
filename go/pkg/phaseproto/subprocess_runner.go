package phaseproto

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// Stable error codes used across the wire. Subprocess agents written in
// other languages MUST agree on these strings; do not rename without a
// version bump.
const (
	CodeHandlerError = "HANDLER_ERROR"
	CodeChildCrashed = "CHILD_CRASHED"
	CodeDecodeFailed = "DECODE_FAILED"
)

// correlationIDFn is a testable seam over CorrelationID minting.
var correlationIDFn = defaultCorrelationID

// SubprocessRunner satisfies core.PhaseRunner by launching a child
// process per call, writing one request envelope to its stdin, and
// reading one response envelope from its stdout.
//
// One process per Run keeps the protocol synchronous and avoids
// cross-cycle state leakage in the child — the orchestrator owns
// lifecycle, the child owns just this one phase invocation.
type SubprocessRunner struct {
	name string
	bin  string
	args []string
	env  []string
}

// NewSubprocessRunner builds a runner that exec.LookPath-style invokes
// `bin args...` with the given extra env. Inherits the parent process
// PATH so callers can pass a name plus relative args.
func NewSubprocessRunner(name, bin string, args, extraEnv []string) *SubprocessRunner {
	return &SubprocessRunner{name: name, bin: bin, args: args, env: extraEnv}
}

// Name returns the phase identity. Matches core.PhaseRunner.Name().
func (r *SubprocessRunner) Name() string { return r.name }

// buildCmd materialises an *exec.Cmd. Broken out for testability.
func (r *SubprocessRunner) buildCmd(ctx context.Context) *exec.Cmd {
	cmd := exec.CommandContext(ctx, r.bin, r.args...)
	cmd.Env = append(os.Environ(), r.env...)
	return cmd
}

// Run launches the child, hands it a request envelope on stdin, and
// reads one response envelope from stdout.
//
// Behaviour summary:
//   - Child returns response → returned to caller.
//   - Child writes error envelope → returned as *WireError.
//   - Child writes garbage / no envelope → CodeDecodeFailed.
//   - Child crashes (non-zero exit) → CodeChildCrashed wrapping exit error.
//   - Context cancelled → child is signalled by exec.CommandContext.
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

	// Parse one envelope from stdout (we only ever speak request/response).
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

// readOneEnvelopeLine pulls one logical line (newline-terminated or EOF)
// from r. Returns the line bytes WITHOUT the trailing newline. An EOF
// reached after some bytes is not an error — io.Reader contract permits
// returning data + io.EOF together.
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

// defaultCorrelationID returns a small unique ID. Doesn't need to be
// cryptographic — orchestrator already tags ledger entries. PID +
// nanos is plenty for matching request/response within one Run.
func defaultCorrelationID() string {
	return fmt.Sprintf("pp-%d", os.Getpid())
}
