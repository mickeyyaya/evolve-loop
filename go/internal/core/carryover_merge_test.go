package core

// carryover_merge_test.go — RED contract for cycle-667 task
// `chronicle-s4-carryover-orphan-merge` (triage `## top_n`, weight 0.92).
//
// ORPHAN BEING FIXED: evolve-memo (the PASS-branch scribe, dispatched post-ship)
// writes <workspace>/carryover-todos.json every PASS cycle, and the retro path
// writes the same file on FAIL — but NO Go code ever reads it. The PASS-branch
// learning channel is fire-and-forget: the queued todos never reach
// state.json:carryoverTodos, so they never surface to the next cycle's planner.
// grep confirms zero readers of any workspace carryover-todos.json.
//
// FIX (Builder authors go/internal/core/carryover_merge.go + wires the call site
// in finalizeCycle): a cycle-terminal hook, beside the persistCycleEndState
// PASS-parity point, that — if <workspace>/carryover-todos.json exists —
// tolerant-decodes [{id, action, priority, evidence_pointer}] (skipping entries
// missing id/action), caps the action via the capRunes idiom, maps priority,
// stamps FirstSeenCycle + a future ExpiresAt (so failurelog.PruneExpiredCarryoverTodos
// can converge), and merges into state.CarryoverTodos via the existing
// mergeCarryoverTodos (dedup by id ⇒ idempotent on re-entry).
//
// These tests are authored by the TDD engineer and are RED now (they will not
// even compile until MergeWorkspaceCarryover exists — a valid RED per the
// compile-failure rule). The Builder must make them GREEN by adding production
// code ONLY; it must NOT modify this file.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive/wiring : TestRunCycle_MergesMemoCarryoverTodosIntoState — the real
//     terminal path (finalizeCycle) persists the memo todos. This is the
//     load-bearing anti-no-op signal: a helper that exists but is never wired
//     into the terminal hook leaves the orphan unfixed and FAILS here.
//   - Semantic       : TestMergeWorkspaceCarryover_DedupesById — re-entry is
//     idempotent, never duplicates an id.
//   - Edge           : TestMergeWorkspaceCarryover_CapsActionRunes — an oversized
//     action is bounded, so a memo todo cannot bloat every future router prompt.
//   - Negative/OOD   : TestMergeWorkspaceCarryover_MalformedFileWarnsNotFails —
//     malformed JSON and id/action-less entries are tolerated (WARN, no panic,
//     no fatal), never aborting the cycle-terminal hook.
//   - Semantic       : TestMergeWorkspaceCarryover_StampsExpiryForPrune — every
//     merged todo carries a future RFC3339 ExpiresAt so the loop-start prune
//     ages the array out instead of letting it grow unboundedly.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// writeMemoCarryover writes a memo-shaped carryover-todos.json (the cycle-646
// live schema: an array of {id, action, priority, evidence_pointer}) into dir.
func writeMemoCarryover(t *testing.T, dir, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "carryover-todos.json"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write carryover-todos.json: %v", err)
	}
}

const memoTwoTodos = `[
  {"id": "todo-alpha", "action": "Design the alpha follow-up", "priority": "high", "evidence_pointer": "scout-report.md#Deferred"},
  {"id": "todo-beta", "action": "Extend the beta contract", "priority": "medium", "evidence_pointer": "scout-report.md#Deferred"}
]`

// TestRunCycle_MergesMemoCarryoverTodosIntoState is the WIRING contract: a cycle
// whose workspace holds a memo-written carryover-todos.json must, at the
// cycle-terminal hook, land those todos in the PERSISTED state — proving the new
// merge is actually called from finalizeCycle (the real terminal path), not just
// defined. A helper that is never wired leaves the orphan open and fails here.
func TestRunCycle_MergesMemoCarryoverTodosIntoState(t *testing.T) {
	ws := t.TempDir()
	writeMemoCarryover(t, ws, memoTwoTodos)

	f := &fakeUpdaterStorage{}
	o := &Orchestrator{
		storage: f,
		gitHEAD: func() (string, error) { return "same-head", nil },
	}

	cs := CycleState{WorkspacePath: ws}
	result := &CycleResult{FinalVerdict: VerdictPASS}
	state := &State{}

	// finalizeCycle is RunCycle's post-loop terminal segment: it persists the
	// cycle-end state via persistCycleEndState. The memo-todo merge belongs here.
	if _, err := o.finalizeCycle(context.Background(), cs, 667, "same-head", result, state); err != nil {
		t.Fatalf("finalizeCycle: %v", err)
	}

	// The persisted state (what the next cycle's planner reads) must carry both
	// memo todos.
	got := f.mem.st.CarryoverTodos
	for _, id := range []string{"todo-alpha", "todo-beta"} {
		if !carryoverTodoExists(got, id) {
			t.Fatalf("RED: memo todo %q not merged into persisted state.CarryoverTodos: %+v\n"+
				"Builder must call MergeWorkspaceCarryover(state, cs.WorkspacePath, cycle, now) "+
				"in finalizeCycle before persistCycleEndState.", id, got)
		}
	}
}

// TestMergeWorkspaceCarryover_DedupesById — re-entry is idempotent. Running the
// merge twice over the same file must not duplicate any id (the orphan-close
// must survive crash-resume / double-invocation).
func TestMergeWorkspaceCarryover_DedupesById(t *testing.T) {
	ws := t.TempDir()
	writeMemoCarryover(t, ws, memoTwoTodos)

	state := &State{}
	MergeWorkspaceCarryover(state, ws, 667, time.Now().UTC())
	afterFirst := len(state.CarryoverTodos)
	MergeWorkspaceCarryover(state, ws, 668, time.Now().UTC())

	if afterFirst != 2 {
		t.Fatalf("RED: first merge added %d todos, want 2", afterFirst)
	}
	if got := len(state.CarryoverTodos); got != 2 {
		t.Fatalf("RED: re-entry duplicated todos: got %d, want 2 (dedup by id must be idempotent): %+v",
			got, state.CarryoverTodos)
	}
}

// TestMergeWorkspaceCarryover_CapsActionRunes — an oversized action is bounded to
// the capRunes ceiling (maxAdoptedDefectRunes) so a memo todo cannot inject an
// arbitrarily large Action that bloats every future router/advisor prompt.
func TestMergeWorkspaceCarryover_CapsActionRunes(t *testing.T) {
	ws := t.TempDir()
	big := strings.Repeat("x", 5000)
	writeMemoCarryover(t, ws, `[{"id":"todo-huge","action":"`+big+`","priority":"low"}]`)

	state := &State{}
	MergeWorkspaceCarryover(state, ws, 667, time.Now().UTC())

	if len(state.CarryoverTodos) != 1 {
		t.Fatalf("RED: expected 1 merged todo, got %d", len(state.CarryoverTodos))
	}
	n := utf8.RuneCountInString(state.CarryoverTodos[0].Action)
	// capRunes(s, maxAdoptedDefectRunes) yields at most maxAdoptedDefectRunes+1
	// runes (the trailing ellipsis marker on truncation).
	if n > maxAdoptedDefectRunes+1 {
		t.Fatalf("RED: Action not capped: got %d runes, want <= %d — apply capRunes(action, maxAdoptedDefectRunes)",
			n, maxAdoptedDefectRunes+1)
	}
	if n >= 5000 {
		t.Fatalf("RED: Action left uncapped at %d runes (input length)", n)
	}
}

// TestMergeWorkspaceCarryover_MalformedFileWarnsNotFails — a corrupt file and
// entries missing id/action are tolerated: no panic, no fatal, no bogus todos.
// The cycle-terminal hook must never abort the cycle over a malformed memo file.
func TestMergeWorkspaceCarryover_MalformedFileWarnsNotFails(t *testing.T) {
	t.Run("corrupt json", func(t *testing.T) {
		ws := t.TempDir()
		writeMemoCarryover(t, ws, `{ this is not valid json `)
		state := &State{}
		MergeWorkspaceCarryover(state, ws, 667, time.Now().UTC()) // must not panic
		if len(state.CarryoverTodos) != 0 {
			t.Fatalf("RED: malformed file produced %d todos, want 0", len(state.CarryoverTodos))
		}
	})

	t.Run("skips entries missing id or action", func(t *testing.T) {
		ws := t.TempDir()
		writeMemoCarryover(t, ws, `[
  {"id": "", "action": "no id here", "priority": "high"},
  {"id": "todo-no-action", "action": "", "priority": "high"},
  {"id": "todo-good", "action": "keep me", "priority": "high"}
]`)
		state := &State{}
		MergeWorkspaceCarryover(state, ws, 667, time.Now().UTC())
		if len(state.CarryoverTodos) != 1 || state.CarryoverTodos[0].ID != "todo-good" {
			t.Fatalf("RED: tolerant decode must skip id/action-less entries, keep only todo-good: %+v",
				state.CarryoverTodos)
		}
	})

	t.Run("absent file is a no-op", func(t *testing.T) {
		ws := t.TempDir() // no carryover-todos.json written
		state := &State{}
		MergeWorkspaceCarryover(state, ws, 667, time.Now().UTC()) // must not panic
		if len(state.CarryoverTodos) != 0 {
			t.Fatalf("RED: absent file must be a no-op, got %+v", state.CarryoverTodos)
		}
	})
}

// TestMergeWorkspaceCarryover_StampsExpiryForPrune — every merged todo carries a
// future RFC3339 ExpiresAt (the same TTL discipline the loop-start backfill uses)
// so failurelog.PruneExpiredCarryoverTodos can age the array out. An unstamped
// todo would never be pruned and the array would grow unboundedly.
func TestMergeWorkspaceCarryover_StampsExpiryForPrune(t *testing.T) {
	ws := t.TempDir()
	writeMemoCarryover(t, ws, memoTwoTodos)

	now := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	state := &State{}
	MergeWorkspaceCarryover(state, ws, 667, now)

	if len(state.CarryoverTodos) == 0 {
		t.Fatal("RED: no todos merged")
	}
	for _, td := range state.CarryoverTodos {
		if td.ExpiresAt == "" {
			t.Fatalf("RED: todo %q left unstamped (ExpiresAt empty) — prune can never age it out", td.ID)
		}
		exp, err := time.Parse(time.RFC3339, td.ExpiresAt)
		if err != nil {
			t.Fatalf("RED: todo %q ExpiresAt %q is not RFC3339: %v", td.ID, td.ExpiresAt, err)
		}
		if !exp.After(now) {
			t.Fatalf("RED: todo %q ExpiresAt %q must be after now %q (future TTL)", td.ID, td.ExpiresAt, now)
		}
		if td.FirstSeenCycle != 667 {
			t.Fatalf("RED: todo %q FirstSeenCycle = %d, want 667 (stamp the merging cycle)", td.ID, td.FirstSeenCycle)
		}
	}
}
