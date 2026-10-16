package inboxmover

import (
	"encoding/json"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"os"
	"path/filepath"
)

const (
	StatePending    = "pending"
	StateProcessing = "processing"
	StateProcessed  = "processed"
	StateConsumed   = "consumed"
	StateRejected   = "rejected"
	StateRetry      = "retry"
	StateQuarantine = "quarantine"
	StateUnknown    = "unknown"
)

var retirementStates = []string{StateConsumed, StateQuarantine, StateProcessed, StateRejected, StateRetry}

type DispatchState struct {
	State  string
	Detail string
	Deps   []string
	Path   string
}

func ResolveDispatchState(opts Options, taskID string) DispatchState {
	opts.resolveOpts()
	if claimDir, held := claimHolding(opts.InboxDir, taskID); held {
		return DispatchState{State: StateProcessing, Detail: claimDir}
	}
	if path, err := FindFileByTaskID(opts.InboxDir, taskID); err == nil {
		return DispatchState{State: StatePending, Deps: readTaskDeps(path), Path: path}
	}
	for _, state := range retirementStates {
		stateDir := filepath.Join(opts.InboxDir, state)
		if _, err := FindFileByTaskID(stateDir, taskID); err == nil {
			return DispatchState{State: state}
		}
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

func claimHolding(inboxDir, taskID string) (string, bool) {
	for _, dir := range inboxbatch.ProcessingCycleDirs(inboxDir) {
		if _, err := FindFileByTaskID(dir, taskID); err == nil {
			return filepath.Base(dir), true
		}
	}
	return "", false
}
