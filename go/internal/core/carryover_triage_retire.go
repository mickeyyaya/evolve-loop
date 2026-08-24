package core

// carryover_triage_retire.go — a todo triage DROPPED must not come back.
//
// cycle-1538: triage recorded
//
//	dropped: [{id: "todo-author-bridge-binding-tests-for-replay-contract-boundary",
//	           reason: "stale: existing bridge tests and ACS replay predicate are green"}]
//
// and the same id was handed to the next cycle's planner anyway. The carryover
// flow has two merge steps at cycle close (MergeWorkspaceCarryover and
// MergeWorkspacePrescriptionCarryover) and no retirement step, so triage's
// decision to STOP working something had nowhere to land: it was written into
// triage-decision.json and read by nobody at the point where state is persisted.
//
// The waste is not one wasted slot. A stale todo that survives retirement is
// re-selected, re-investigated, and re-dropped every cycle — triage pays the
// judgement cost again each time and reaches the same conclusion, because the
// conclusion was never recorded anywhere the next cycle could see it.
//
// The reader lives HERE rather than beside cycleoutcome.CommittedIDsFor, which
// is the natural neighbour: core cannot import cycleoutcome
// (core → cycleoutcome → inboxmover → adapters/ledger → core), and it cannot
// import triagecap either (triagecap imports core). So this is the one reader
// the one consumer can actually reach. triagecap owns the WRITE shape; this
// owns the READ. Keep it that way — a third parser is how "committed" and
// "dropped" drift apart on the same document.
// (2026-08-24 amendment: inboxmover.DroppedIDs now reads the same dropped[]
// field for SHIP CONSUMPTION — forced apart from this reader by the very
// import cycle above. A schema change to dropped[] must land in BOTH readers
// or consumption and carryover retirement diverge on the same document.)

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// triageDroppedIDs reads a cycle workspace's triage-decision.json and returns
// the ids triage explicitly DROPPED — the set it decided are no longer worth
// working (stale, already satisfied, superseded).
//
// nil on any absent/unreadable/malformed decision. Continuation and lane cycles
// carry no triage-decision.json at all, and they must retire nothing.
func triageDroppedIDs(workspace string) []string {
	body, err := os.ReadFile(filepath.Join(workspace, "triage-decision.json"))
	if err != nil {
		return nil
	}
	var d struct {
		Dropped []struct {
			ID string `json:"id"`
		} `json:"dropped"`
	}
	if json.Unmarshal(body, &d) != nil {
		return nil
	}
	var out []string
	for _, e := range d.Dropped {
		if id := strings.TrimSpace(e.ID); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// retireTriageDroppedCarryover removes every carryover todo whose id appears in
// this cycle's triage-decision.json `dropped` list.
//
// Deliberately narrow: ONLY the explicit dropped set. `deferred` is triage
// saying "not this cycle" and must survive; absent/malformed decisions retire
// nothing. Forgetting a live todo is worse than carrying a stale one, so every
// ambiguous case keeps the todo.
func retireTriageDroppedCarryover(state *State, workspace string) {
	if state == nil || workspace == "" || len(state.CarryoverTodos) == 0 {
		return
	}
	dropped := triageDroppedIDs(workspace)
	if len(dropped) == 0 {
		return
	}
	retire := make(map[string]bool, len(dropped))
	for _, id := range dropped {
		retire[id] = true
	}
	kept := make([]CarryoverTodo, 0, len(state.CarryoverTodos))
	for _, t := range state.CarryoverTodos {
		if !retire[t.ID] {
			kept = append(kept, t)
		}
	}
	state.CarryoverTodos = kept
}
