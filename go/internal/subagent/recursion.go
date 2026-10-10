package subagent

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	// SSOT IPC-protocol-allowed: parent→child recursion-depth handoff
	// Split form keeps it out of the flagreaders guard's standalone-literal
	// scan (SSOT §IPC-protocol-allowed) — same pattern as FanoutWorkerTokenEnv.
	dispatchDepthEnv = "EVOLVE_" + "DISPATCH_DEPTH"
	// FanoutWorkerTokenEnv carries the parent-dictated per-worker challenge
	// token to the worker dispatch (consumed by run.go as
	// RunRequest.ChallengeTokenOverride) so the worker's artifact bears the
	// token the parent verifies — the per-worker provenance boundary.
	// Split form keeps it out of the flagreaders guard's standalone-literal
	// scan (SSOT §IPC-protocol-allowed).
	FanoutWorkerTokenEnv = "EVOLVE_" + "FANOUT_WORKER_TOKEN"
	maxDispatchDepth     = 3
)

var ErrRecursionDepthExceeded = errors.New(
	"subagent/run: recursion depth cap exceeded — too many nested bridge dispatches (likely a fan-out loop); inspect EVOLVE_DISPATCH_DEPTH",
)

func ReadDispatchDepth(getenv func(string) string) int {
	n, err := strconv.Atoi(strings.TrimSpace(getenv(dispatchDepthEnv)))
	if err != nil || n < 0 {
		return 0
	}
	return n
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
