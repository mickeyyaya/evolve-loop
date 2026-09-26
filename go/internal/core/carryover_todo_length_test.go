package core_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// boilerplatePrefix is a literal so the test breaks loudly if the sentence drifts.
const boilerplatePrefix = "Review the failed cycle learning and fix before retrying:"

// maxDefectActionRunes allows the 500-rune cap plus the "Fix defect from cycle N: " prefix.
const maxDefectActionRunes = 600

func TestApplyDefectsAsCarryoverTodosBoundsLength(t *testing.T) {
	huge := strings.Repeat("x", 5000)
	state := &core.State{}
	record := core.FailedRecord{
		Cycle:   488,
		Verdict: "FAIL",
		Defects: []string{huge},
	}
	core.ApplyDefectsAsCarryoverTodos(state, record)

	if len(state.CarryoverTodos) == 0 {
		t.Fatal("expected at least one carryover todo for a non-blank defect")
	}
	for _, todo := range state.CarryoverTodos {
		if n := len([]rune(todo.Action)); n > maxDefectActionRunes {
			t.Errorf("Action from a 5000-rune defect is unbounded: got %d runes, want <= %d — ApplyDefectsAsCarryoverTodos must cap defect text like failureLearningSummary does",
				n, maxDefectActionRunes)
		}
	}
}

func TestApplyDefectsAsCarryoverTodos_ShortDefectPreserved(t *testing.T) {
	const shortDefect = "nil pointer in router when signals absent"
	state := &core.State{}
	record := core.FailedRecord{
		Cycle:   488,
		Verdict: "FAIL",
		Defects: []string{shortDefect},
	}
	core.ApplyDefectsAsCarryoverTodos(state, record)

	if len(state.CarryoverTodos) != 1 {
		t.Fatalf("want exactly 1 todo for one short defect, got %d", len(state.CarryoverTodos))
	}
	if !strings.Contains(state.CarryoverTodos[0].Action, shortDefect) {
		t.Errorf("short defect must be preserved verbatim in the Action; got %q", state.CarryoverTodos[0].Action)
	}
}

func TestCarryoverTodoActionDropsBoilerplatePrefix(t *testing.T) {
	root := t.TempDir()
	seedCycleStateFile(t, root)

	orch, st, _ := newTestOrchestrator(t, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseTriage: &alwaysErrRunner{name: "triage"},
		core.PhaseRetro:  &alwaysErrRunner{name: "retro"},
	}))
	if _, err := orch.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: root,
		GoalHash:    "test-goal",
		Context:     map[string]string{"commit_message": "test commit"},
	}); err == nil {
		t.Fatal("triage hard failure must surface as a cycle error")
	}

	var todo *core.CarryoverTodo
	for i := range st.state.CarryoverTodos {
		if strings.HasPrefix(st.state.CarryoverTodos[i].ID, "cycle-1-failed-") {
			todo = &st.state.CarryoverTodos[i]
			break
		}
	}
	if todo == nil {
		t.Fatalf("recordFailureLearning must queue a cycle-1-failed-* carryover todo; got %+v", st.state.CarryoverTodos)
	}
	if strings.Contains(todo.Action, boilerplatePrefix) {
		t.Errorf("Action still carries the redundant boilerplate prefix %q (the router-prompt section header already states it); Action=%q",
			boilerplatePrefix, todo.Action)
	}
	if !strings.Contains(todo.Action, "cycle 1") || !strings.Contains(todo.Action, "triage") {
		t.Errorf("Action must retain cycle/phase info after prefix removal; got %q", todo.Action)
	}
}
