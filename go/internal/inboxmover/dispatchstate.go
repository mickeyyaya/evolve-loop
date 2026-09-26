package inboxmover

import (
	"encoding/json"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"os"
	"path/filepath"
)

// Dispatch states reported by ResolveDispatchState; Unknown means no lifecycle evidence.
const (
	StatePending    = "pending"
	StateProcessing = "processing"
	StateProcessed  = "processed"
	StateConsumed   = "consumed" // `evolve inbox consume`: the in-commit landing consumption
	StateRejected   = "rejected"
	StateRetry      = "retry"
	StateQuarantine = "quarantine"
	StateUnknown    = "unknown"
)

// retirementStates is the one list every "has this id retired?" reader walks, in
// resolution order. The dispatch gate fails open on unknown, so a missing state relaunches a retired item.
var retirementStates = []string{StateConsumed, StateQuarantine, StateProcessed, StateRejected, StateRetry}

// DispatchState is one task id's current lifecycle position.
type DispatchState struct {
	State  string   // one of the State* constants
	Detail string   // the cycle dir name (cycle-N) when the state has one
	Deps   []string // declared deps, pending only
	// Path is the live record's path, pending only. Ids repeat across consumed/
	// namesakes, so a bare id is unsafe to hand to an agent.
	Path string
}

// ResolveDispatchState classifies taskID by the lifecycle dir holding it; unreadable evidence counts as none.
func ResolveDispatchState(opts Options, taskID string) DispatchState {
	opts.resolveOpts()
	if path, err := FindFileByTaskID(opts.InboxDir, taskID); err == nil {
		return DispatchState{State: StatePending, Deps: readTaskDeps(path), Path: path}
	}
	for _, dir := range inboxbatch.ProcessingCycleDirs(opts.InboxDir) {
		if _, err := FindFileByTaskID(dir, taskID); err == nil {
			return DispatchState{State: StateProcessing, Detail: filepath.Base(dir)}
		}
	}
	for _, state := range retirementStates {
		stateDir := filepath.Join(opts.InboxDir, state)
		if _, err := FindFileByTaskID(stateDir, taskID); err == nil {
			return DispatchState{State: state}
		}
		// Newest cycle first: a hit is almost always recent, and the nested trees only grow.
		dirs := inboxbatch.CycleDirs(stateDir)
		for i := len(dirs) - 1; i >= 0; i-- {
			if _, err := FindFileByTaskID(dirs[i], taskID); err == nil {
				return DispatchState{State: state, Detail: filepath.Base(dirs[i])}
			}
		}
	}
	return DispatchState{State: StateUnknown}
}

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
