package core

import (
	"context"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

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

	if _, err := o.finalizeCycle(context.Background(), cs, 667, "same-head", "", result, state, nil); err != nil {
		t.Fatalf("finalizeCycle: %v", err)
	}

	got := f.mem.st.CarryoverTodos
	for _, id := range []string{"todo-alpha", "todo-beta"} {
		if !carryoverTodoExists(got, id) {
			t.Fatalf("RED: memo todo %q not merged into persisted state.CarryoverTodos: %+v\n"+
				"Builder must call MergeWorkspaceCarryover(state, cs.WorkspacePath, cycle, now) "+
				"in finalizeCycle before persistCycleEndState.", id, got)
		}
	}
}

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
	// The cap adds one rune: the truncation ellipsis.
	if n > carryover.MaxActionRunes+1 {
		t.Fatalf("RED: Action not capped: got %d runes, want <= %d — apply capRunes(action, carryover.MaxActionRunes)",
			n, carryover.MaxActionRunes+1)
	}
	if n >= 5000 {
		t.Fatalf("RED: Action left uncapped at %d runes (input length)", n)
	}
}

func TestMergeWorkspaceCarryover_MalformedFileWarnsNotFails(t *testing.T) {
	t.Run("wired: the malformed memo is a CARRYOVER_WORKSPACE_MALFORMED signal", func(t *testing.T) {
		ws := t.TempDir()
		writeMemoCarryover(t, ws, `{ this is not valid json `)
		signals, got := recordingCenter()
		o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(signals))
		state := &State{}
		o.carryover().MergeMemo(state, ws, 667, time.Now().UTC())
		warned := eventsOfKind(*got, signalcenter.KindCarryoverWarning)
		if len(state.CarryoverTodos) != 0 || len(warned) != 1 || warned[0].Code != carryover.CodeWorkspaceMalformed || warned[0].Origin != "Lifecycle.MergeMemo" {
			t.Fatalf("the wired lifecycle reports the malformed memo once and merges nothing: %+v", *got)
		}
	})

	t.Run("corrupt json", func(t *testing.T) {
		ws := t.TempDir()
		writeMemoCarryover(t, ws, `{ this is not valid json `)
		state := &State{}
		MergeWorkspaceCarryover(state, ws, 667, time.Now().UTC())
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
		MergeWorkspaceCarryover(state, ws, 667, time.Now().UTC())
		if len(state.CarryoverTodos) != 0 {
			t.Fatalf("RED: absent file must be a no-op, got %+v", state.CarryoverTodos)
		}
	})
}

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
