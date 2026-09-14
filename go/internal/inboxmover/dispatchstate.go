package inboxmover

// dispatchstate.go — dispatch-time task-state resolution for the fleet
// freshness gate (cycle 767, inbox id dispatch-freshness-gate). The gate must
// re-resolve a planned task id against the CURRENT inbox lifecycle immediately
// before lane launch; this package owns the lifecycle dirs, so the resolver
// lives here and the fleet/cmd layers stay lifecycle-layout-agnostic.

import (
	"encoding/json"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"os"
	"path/filepath"
)

// Dispatch states reported by ResolveDispatchState. Pending means the task is
// still launchable; Processing/Processed/Rejected/Retry/Quarantine mirror the
// lifecycle dirs; Unknown means no lifecycle evidence exists (callers fail open
// — not every planned id is inbox-backed).
const (
	StatePending    = "pending"
	StateProcessing = "processing"
	StateProcessed  = "processed"
	StateConsumed   = "consumed" // `evolve inbox consume`: the in-commit landing consumption (tracked, rides the ship)
	StateRejected   = "rejected"
	StateRetry      = "retry"
	StateQuarantine = "quarantine"
	StateUnknown    = "unknown"
)

// retirementStates are the lifecycle states an item lands in when it LEAVES
// the pending pool, in resolution order (first hit wins when an id sits in
// two dirs). The ONE list for every reader that asks "has this id retired?":
// ResolveDispatchState here and scopeRetiredAt (continuation_retire.go).
// processed/ and rejected/ nest a cycle-<N> level (lifecycle.promoteDestPath);
// every state is scanned flat AND nested so no layout change can hide a
// retired item from one reader but not the other (cycle 1682: a shipped item
// resolved unknown, survived both prunes and the launch probe, and was
// re-pinned to a lane).
var retirementStates = []string{StateConsumed, StateQuarantine, StateProcessed, StateRejected, StateRetry}

// DispatchState is one task id's current lifecycle position.
type DispatchState struct {
	State  string   // one of the State* constants
	Detail string   // e.g. "cycle-748" when State==StateProcessing
	Deps   []string // declared deps (populated only when State==StatePending)
	// Path is the LIVE record's absolute path, populated only for
	// StatePending. Auto-minted ids are deliberately stable per category (the
	// dedup identity), so consumed/ accumulates same-id namesakes forever and
	// a bare id is structurally unsafe to hand to an agent: cycle-1548 worked
	// an already-cured two-week-old consumed record because its prompt carried
	// only the name. The resolver had this path in hand all along; carrying it
	// is what lets dispatch disclose the one correct file.
	Path string
}

// ResolveDispatchState classifies taskID against the inbox lifecycle dirs:
// inbox/ → pending (with its declared deps), processing/cycle-N/ → processing
// (Detail names the cycle), processed|rejected|retry|quarantine/ → that state,
// and no evidence anywhere → unknown.
//
// quarantine/ is load-bearing here, not decorative: without it a todo parked by
// the ADR-0072 S5 retry ceiling fell through to StateUnknown, which the
// dispatch freshness gate fails OPEN on — so the ceiling would park a poison
// todo and the very next wave would launch it again. Best-effort reads throughout — an unreadable
// dir or malformed file is treated as no evidence, never an error, so a bad
// inbox can only ever fail OPEN at the dispatch gate.
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
		// Newest cycle first: a hit is almost always recent (1682 chased
		// 1679) while the nested trees grow one dir per cycle for good.
		dirs := inboxbatch.CycleDirs(stateDir)
		for i := len(dirs) - 1; i >= 0; i-- {
			if _, err := FindFileByTaskID(dirs[i], taskID); err == nil {
				return DispatchState{State: state, Detail: filepath.Base(dirs[i])}
			}
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
