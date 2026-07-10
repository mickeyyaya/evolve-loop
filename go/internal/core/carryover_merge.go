package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
)

// memoCarryoverTodo mirrors the on-disk schema evolve-memo (the PASS-branch
// scribe, dispatched post-ship) and the retro path both write to
// <workspace>/carryover-todos.json — the cycle-646 live schema: an array of
// {id, action, priority, evidence_pointer}. Only id/action/priority are consumed;
// evidence_pointer is decoded for forward-compat but not (yet) stored on the todo.
type memoCarryoverTodo struct {
	ID              string `json:"id"`
	Action          string `json:"action"`
	Priority        string `json:"priority"`
	EvidencePointer string `json:"evidence_pointer"`
}

// MergeWorkspaceCarryover closes the PASS-branch learning orphan (chronicle-s4):
// evolve-memo and the retro path both write <workspace>/carryover-todos.json, but
// no Go code ever read it, so the queued follow-up todos never reached
// state.json:carryoverTodos and never surfaced to the next cycle's planner. This
// cycle-terminal hook (wired in finalizeCycle beside persistCycleEndState)
// tolerant-decodes that file and merges its todos into state.CarryoverTodos.
//
// A missing file is a no-op. A malformed file — or an entry missing id/action —
// is tolerated: it WARNs to stderr and is skipped, never aborting the cycle. Each
// merged todo's Action is capped via capRunes(action, maxAdoptedDefectRunes) so a
// memo cannot bloat every future router/advisor prompt, and each carries
// FirstSeenCycle + a future ExpiresAt (the same DefaultCarryoverBackfillTTL the
// loop-start backfill stamps) so failurelog.PruneExpiredCarryoverTodos can age the
// array out instead of letting it grow unboundedly. Dedup is by id via the
// existing mergeCarryoverTodos, so re-entry (crash-resume / double-invocation) is
// idempotent.
func MergeWorkspaceCarryover(state *State, workspacePath string, cycle int, now time.Time) {
	if state == nil || strings.TrimSpace(workspacePath) == "" {
		return
	}
	path := filepath.Join(workspacePath, "carryover-todos.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN carryover-merge: read %s: %v\n", path, err)
		}
		return // absent file is a no-op
	}
	var memo []memoCarryoverTodo
	if err := json.Unmarshal(raw, &memo); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN carryover-merge: malformed %s (skipping): %v\n", path, err)
		return
	}

	// Same default TTL the loop-start backfill stamps (failurelog.BackfillLegacyCarryoverExpiry),
	// so the two carryover-todo write paths share one aging discipline.
	expiresAt := now.Add(failurelog.DefaultCarryoverBackfillTTL).Format(time.RFC3339)
	incoming := make([]CarryoverTodo, 0, len(memo))
	for _, m := range memo {
		id := strings.TrimSpace(m.ID)
		action := strings.TrimSpace(m.Action)
		if id == "" || action == "" {
			continue // tolerant decode: skip id/action-less entries
		}
		priority := strings.TrimSpace(m.Priority)
		if priority == "" {
			priority = "medium"
		}
		incoming = append(incoming, CarryoverTodo{
			ID:             id,
			Action:         capRunes(action, maxAdoptedDefectRunes),
			Priority:       priority,
			FirstSeenCycle: cycle,
			ExpiresAt:      expiresAt,
		})
	}
	state.CarryoverTodos = mergeCarryoverTodos(state.CarryoverTodos, incoming)
}
