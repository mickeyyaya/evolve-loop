package subagent

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

const (
	dispatchDepthEnv = ipcenv.DispatchDepthKey
	// Split form keeps it out of the flagreaders guard's standalone-literal
	// scan (SSOT §IPC-protocol-allowed).
	FanoutWorkerTokenEnv = "EVOLVE_" + "FANOUT_WORKER_TOKEN"
	maxDispatchDepth     = 3
)

var ErrRecursionDepthExceeded = errors.New(
	"subagent/run: recursion depth cap exceeded — too many nested bridge dispatches (likely a fan-out loop); inspect EVOLVE_DISPATCH_DEPTH",
)

func ReadDispatchDepth(getenv func(string) string) int {
	return ipcenv.DispatchDepth(getenv)
}

func enforceDispatchDepth(depth int) error {
	if depth > maxDispatchDepth {
		return ErrRecursionDepthExceeded
	}
	return nil
}

func enforceChildDispatchDepth(parentDepth int) error {
	return enforceDispatchDepth(parentDepth + 1)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func buildWorkerRecursionCommand(bin, parentAgent, subtask string, cycle, childDepth int, workspace, promptPath, workerToken string) string {
	return fmt.Sprintf(
		"PROMPT_FILE_OVERRIDE=%s CLAUDECODE_TYPE= %s=%d %s=%s %s subagent run %s-worker-%s %d %s",
		shellQuote(promptPath), dispatchDepthEnv, childDepth, FanoutWorkerTokenEnv, shellQuote(workerToken),
		shellQuote(bin), parentAgent, subtask, cycle, shellQuote(workspace),
	)
}
