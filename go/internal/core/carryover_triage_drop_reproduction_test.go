package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestFinalizeCycle_RetiresTriageDroppedCarryover reproduces cycle 1538: a
// no-ship WARN cycle records a stale carryover in triage-decision.json, but the
// terminal path persists that same carryover for the next cycle.
func TestFinalizeCycle_RetiresTriageDroppedCarryover(t *testing.T) {
	const droppedID = "todo-author-bridge-binding-tests-for-replay-contract-boundary"

	workspace := t.TempDir()
	decision := `{
  "cycle": 1538,
  "top_n": [],
  "dropped": [{
    "id": "todo-author-bridge-binding-tests-for-replay-contract-boundary",
    "reason": "stale: existing bridge tests and ACS replay predicate are green"
  }]
}`
	if err := os.WriteFile(filepath.Join(workspace, "triage-decision.json"), []byte(decision), 0o644); err != nil {
		t.Fatalf("write triage decision: %v", err)
	}

	storage := &fakeUpdaterStorage{}
	orchestrator := &Orchestrator{
		storage: storage,
		gitHEAD: func() (string, error) { return "same-head", nil },
	}
	state := &State{CarryoverTodos: []CarryoverTodo{
		{ID: droppedID, Action: "author bridge binding tests"},
		{ID: "still-live", Action: "fix a different active defect"},
	}}
	result := &CycleResult{FinalVerdict: VerdictWARN}

	if _, err := orchestrator.finalizeCycle(context.Background(), CycleState{WorkspacePath: workspace}, 1538, "same-head", "", result, state, nil); err != nil {
		t.Fatalf("finalizeCycle: %v", err)
	}

	persisted := storage.mem.st.CarryoverTodos
	if carryoverTodoExists(persisted, droppedID) {
		t.Fatalf("stale triage-dropped carryover %q survived finalizeCycle: %+v", droppedID, persisted)
	}
	if !carryoverTodoExists(persisted, "still-live") {
		t.Fatalf("unrelated live carryover was removed: %+v", persisted)
	}
}
