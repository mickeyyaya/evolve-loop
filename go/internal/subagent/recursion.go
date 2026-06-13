package subagent

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// recursion.go — single source of truth for the bridge-recursion contract:
// fan-out workers re-enter the SAME evolve binary via `subagent run` (never the
// in-process Agent tool — see [B1] enforceBridgeOnly), and that recursion is
// bounded + sandbox-coherent at every depth.
//
// Why a depth cap: a worker runs `evolve subagent run <agent>-worker-<subtask>`,
// which is another full Run() — and a worker could itself fan out. Without a cap
// a misconfigured profile could nest dispatches unboundedly. EVOLVE_DISPATCH_DEPTH
// threads the depth across each bridge re-entry; Run() rejects past the cap.
//
// Why clear CLAUDECODE_TYPE: a recursive child is, by definition, NOT the
// top-level host. If a stale CLAUDECODE_TYPE=host leaked into a child it would
// make sandbox.DetectNested return false → ShouldWrap would attempt the inner OS
// sandbox → on macOS sandbox_apply() EPERMs and hangs the REPL boot (the Part A
// failure mode). Clearing it for every worker keeps DetectNested=true by
// construction at any depth.

const (
	dispatchDepthEnv = "EVOLVE_DISPATCH_DEPTH"
	// fanoutWorkerTokenEnv carries the parent-dictated per-worker challenge
	// token to the worker dispatch (consumed by run.go as
	// RunRequest.ChallengeTokenOverride) so the worker's artifact bears the
	// token the parent verifies — the per-worker provenance boundary.
	fanoutWorkerTokenEnv = "EVOLVE_FANOUT_WORKER_TOKEN"
	// maxDispatchDepth bounds nested bridge dispatches. Normal recursion is
	// fan-out → worker (depth 1); a generous cap of 3 backstops pathological
	// loops without constraining legitimate nesting.
	maxDispatchDepth = 3
)

// ErrRecursionDepthExceeded is returned when a dispatch's recursion depth
// exceeds maxDispatchDepth — almost always a fan-out loop.
var ErrRecursionDepthExceeded = errors.New(
	"subagent/run: recursion depth cap exceeded — too many nested bridge dispatches (likely a fan-out loop); inspect EVOLVE_DISPATCH_DEPTH",
)

// ReadDispatchDepth parses the current recursion depth from the environment.
// Absent / invalid / negative ⇒ 0 (top-level). getenv is injected (os.Getenv
// in production, a map in tests).
func ReadDispatchDepth(getenv func(string) string) int {
	n, err := strconv.Atoi(strings.TrimSpace(getenv(dispatchDepthEnv)))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// enforceDispatchDepth rejects a dispatch RUNNING deeper than the cap. Called at
// the Run() entry point (a worker re-entered here at its own depth).
func enforceDispatchDepth(depth int) error {
	if depth > maxDispatchDepth {
		return ErrRecursionDepthExceeded
	}
	return nil
}

// enforceChildDispatchDepth rejects FANNING OUT when the workers (which run at
// parentDepth+1) would exceed the cap — the fail-fast check for DispatchParallel,
// so it refuses before spawning doomed workers rather than letting each worker's
// Run() reject. Both predicates share maxDispatchDepth (one cap, two fences).
func enforceChildDispatchDepth(parentDepth int) error {
	return enforceDispatchDepth(parentDepth + 1)
}

// shellQuote single-quotes a string for safe inclusion in a `/bin/sh -c` command.
// An embedded single quote is escaped by closing the quote, emitting a
// backslash-escaped quote, then reopening (the standard POSIX idiom). Worker
// recursion commands interpolate filesystem paths, which may contain spaces or
// shell metacharacters.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// buildWorkerRecursionCommand composes the shell command a fan-out worker runs
// to recurse into the same evolve binary via the bridge dispatch path. It
// threads the child recursion depth and clears the host marker so the child is
// always detected as nested (no inner sandbox wrap).
//
//	bin          path to the evolve binary (os.Executable of the parent)
//	parentAgent  the fanning-out agent role (e.g. "auditor")
//	subtask      the worker subtask name (composes "<role>-worker-<subtask>")
//	cycle        the cycle number, passed through unchanged
//	childDepth   the recursion depth the worker will run at (parentDepth+1)
//	workspace    the workspace path, passed through unchanged
//	promptPath   the per-worker rendered prompt file
//	workerToken  the parent-dictated challenge token the worker must write into
//	             its artifact (parentToken+"-"+subtask) — the provenance the
//	             parent verifies; threaded via EVOLVE_FANOUT_WORKER_TOKEN
func buildWorkerRecursionCommand(bin, parentAgent, subtask string, cycle, childDepth int, workspace, promptPath, workerToken string) string {
	// Paths + token are shell-quoted; role/subtask (regex-constrained) and the
	// int cycle/depth are safe unquoted.
	return fmt.Sprintf(
		"PROMPT_FILE_OVERRIDE=%s CLAUDECODE_TYPE= %s=%d %s=%s %s subagent run %s-worker-%s %d %s",
		shellQuote(promptPath), dispatchDepthEnv, childDepth, fanoutWorkerTokenEnv, shellQuote(workerToken),
		shellQuote(bin), parentAgent, subtask, cycle, shellQuote(workspace),
	)
}
