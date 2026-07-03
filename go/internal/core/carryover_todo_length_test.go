package core_test

// carryover_todo_length_test.go — cycle-488 RED tests for
// tighten-carryover-todo-creation-length (Task 1).
//
// Two creation paths write a router.CarryoverTodo.Action that later renders
// into EVERY router/advisor prompt (phase_advisor.writeCarryoverTodos). Today
// they are asymmetric and lossy:
//
//   1. recordFailureLearning (failure_learning.go ~L246) prepends the fixed
//      58-byte boilerplate "Review the failed cycle learning and fix before
//      retrying: " to every todo — pure repetition the router prompt's own
//      section header already states.
//   2. ApplyDefectsAsCarryoverTodos (failure_learning.go ~L535) applies NO
//      length cap to `defect`, unlike its sibling failureLearningSummary
//      (maxFailureLearningSummaryChars = 500). A single unbounded audit-gate
//      diagnostic (e.g. a long strings.Join(offenders, "; ")) can inject an
//      arbitrarily large Action that bloats every future router prompt.
//
// These tests encode the acceptance criteria. They MUST fail before the Builder
// touches failure_learning.go (RED). The Builder makes them GREEN by dropping
// the prefix and bounding the defect text — WITHOUT modifying this file.

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// boilerplatePrefix is the redundant sentence Task 1 removes from every
// recordFailureLearning-created todo. Kept as a literal here so the test breaks
// loudly if the string drifts.
const boilerplatePrefix = "Review the failed cycle learning and fix before retrying:"

// maxDefectActionRunes is the RED bound: a 5000-rune synthetic defect must be
// bounded well under this ceiling. The sibling cap is 500 runes; the Action
// wraps the (capped) defect in a short "Fix defect from cycle N: " prefix, so a
// correct fix lands near ~530 runes. 600 leaves the Builder room to pick the
// exact cap while still proving the 5000→bounded reduction (a no-op renders
// ~5026 runes and fails). See scout Hypothesis 2.
const maxDefectActionRunes = 600

// TestApplyDefectsAsCarryoverTodosBoundsLength (Task1-AC: ApplyDefects applies
// the same/equivalent length bound as failureLearningSummary). Feeds a
// 5000-rune defect and asserts every generated Action is bounded. RED today:
// ApplyDefectsAsCarryoverTodos has no cap, so Action ≈ 5026 runes.
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

// TestApplyDefectsAsCarryoverTodos_ShortDefectPreserved (boundary / semantic
// diversity): the cap must TRUNCATE only oversized defects — a short defect
// must survive intact so the todo stays actionable. Guards against a fix that
// over-truncates every defect. Pre-existing GREEN; its value is surviving the
// Builder's edit.
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

// TestCarryoverTodoActionDropsBoilerplatePrefix (Task1-AC: recordFailureLearning
// todos no longer carry the fixed boilerplate prefix while retaining
// cycle/phase/error-class info). Drives a full cycle whose triage hard-fails
// (and whose retro also fails, exercising the failure-learning queue path),
// then inspects the queued carryover todo. RED today: the Action still begins
// with the boilerplate sentence.
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
	// The prefix is redundant, but the cycle/phase/error-class summary is NOT —
	// dropping the prefix must not drop the signal the router actually needs.
	if !strings.Contains(todo.Action, "cycle 1") || !strings.Contains(todo.Action, "triage") {
		t.Errorf("Action must retain cycle/phase info after prefix removal; got %q", todo.Action)
	}
}
