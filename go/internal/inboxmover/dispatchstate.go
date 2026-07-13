package inboxmover

// dispatchstate.go — dispatch-time task-state resolution for the fleet
// freshness gate (cycle 767, inbox id dispatch-freshness-gate). The gate must
// re-resolve a planned task id against the CURRENT inbox lifecycle immediately
// before lane launch; this package owns the lifecycle dirs, so the resolver
// lives here and the fleet/cmd layers stay lifecycle-layout-agnostic.

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Dispatch states reported by ResolveDispatchState. Pending means the task is
// still launchable; Processing/Processed/Rejected/Retry mirror the lifecycle
// dirs; Unknown means no lifecycle evidence exists (callers fail open — not
// every planned id is inbox-backed).
const (
	StatePending    = "pending"
	StateProcessing = "processing"
	StateProcessed  = "processed"
	StateRejected   = "rejected"
	StateRetry      = "retry"
	StateUnknown    = "unknown"
)

// DispatchState is one task id's current lifecycle position.
type DispatchState struct {
	State  string   // one of the State* constants
	Detail string   // e.g. "cycle-748" when State==StateProcessing
	Deps   []string // declared deps (populated only when State==StatePending)
}

// ResolveDispatchState classifies taskID against the inbox lifecycle dirs:
// inbox/ → pending (with its declared deps), processing/cycle-N/ → processing
// (Detail names the cycle), processed|rejected|retry/ → that state, and no
// evidence anywhere → unknown. Best-effort reads throughout — an unreadable
// dir or malformed file is treated as no evidence, never an error, so a bad
// inbox can only ever fail OPEN at the dispatch gate.
func ResolveDispatchState(opts Options, taskID string) DispatchState {
	opts.resolveOpts()
	if path, err := findFileByTaskID(opts.InboxDir, taskID); err == nil {
		return DispatchState{State: StatePending, Deps: readTaskDeps(path)}
	}
	cycles, _ := filepath.Glob(filepath.Join(opts.InboxDir, "processing", "cycle-*"))
	for _, dir := range cycles {
		if _, err := findFileByTaskID(dir, taskID); err == nil {
			return DispatchState{State: StateProcessing, Detail: filepath.Base(dir)}
		}
	}
	for _, state := range []string{StateProcessed, StateRejected, StateRetry} {
		if _, err := findFileByTaskID(filepath.Join(opts.InboxDir, state), taskID); err == nil {
			return DispatchState{State: state}
		}
	}
	return DispatchState{State: StateUnknown}
}

// readTaskDeps returns the task file's declared "deps" ids (empty on any
// read/parse failure — best-effort, same posture as the rest of the package).
func readTaskDeps(path string) []string {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc struct {
		Deps []string `json:"deps"`
	}
	if json.Unmarshal(body, &doc) != nil {
		return nil
	}
	return doc.Deps
}
