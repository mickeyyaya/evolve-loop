package core_test

// failure_learning_amplify_test.go — Test Amplification phase adversarial
// cases for ApplyDefectsAsCarryoverTodos. Covers edge cases NOT in the
// TDD-engineer RED tests: blank defects, idempotency, deterministic IDs,
// priority/field contracts, and multi-defect isolation.
//
// GAP TESTS are marked explicitly; they expose contract requirements
// the current implementation does not yet satisfy.

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestApplyDefects_BlankDefectIgnored: GAP — contract says "Blank defects are
// ignored". Current implementation uses a raw index loop without TrimSpace,
// so blank strings DO produce todos (ID = "cycle-N-defect-0").
func TestApplyDefects_BlankDefectIgnored(t *testing.T) {
	state := &core.State{}
	record := core.FailedRecord{
		Cycle:   3,
		Verdict: "FAIL",
		Defects: []string{"", "   ", "\t"},
	}
	core.ApplyDefectsAsCarryoverTodos(state, record)
	if len(state.CarryoverTodos) != 0 {
		t.Errorf("GAP: blank/whitespace defects must not produce todos (contract: 'Blank defects are ignored'); got %d",
			len(state.CarryoverTodos))
	}
}

// TestApplyDefects_MixedBlankAndRealDefects: GAP — blank defects should be
// skipped while real ones produce todos. Current implementation does not filter.
func TestApplyDefects_MixedBlankAndRealDefects(t *testing.T) {
	state := &core.State{}
	record := core.FailedRecord{
		Cycle:   4,
		Verdict: "FAIL",
		Defects: []string{"", "real defect A", "   ", "real defect B"},
	}
	core.ApplyDefectsAsCarryoverTodos(state, record)
	// Expect exactly 2 todos (one per real defect, blanks skipped).
	if len(state.CarryoverTodos) != 2 {
		t.Errorf("GAP: want 2 todos (blanks skipped), got %d", len(state.CarryoverTodos))
	}
	for _, defect := range []string{"real defect A", "real defect B"} {
		found := false
		for _, todo := range state.CarryoverTodos {
			if strings.Contains(todo.Action, defect) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no todo found for defect %q", defect)
		}
	}
}

// TestApplyDefects_Idempotent: calling ApplyDefectsAsCarryoverTodos twice with
// the same record must not duplicate existing todos.
// Contract: "Replaying failure learning is idempotent."
func TestApplyDefects_Idempotent(t *testing.T) {
	state := &core.State{}
	record := core.FailedRecord{
		Cycle:   5,
		Verdict: "FAIL",
		Defects: []string{"idempotency defect one", "idempotency defect two"},
	}
	core.ApplyDefectsAsCarryoverTodos(state, record)
	firstCount := len(state.CarryoverTodos)
	if firstCount == 0 {
		t.Fatal("first apply produced no todos; cannot verify idempotency")
	}
	core.ApplyDefectsAsCarryoverTodos(state, record)
	if len(state.CarryoverTodos) != firstCount {
		t.Errorf("idempotency violated: count changed from %d to %d on second apply",
			firstCount, len(state.CarryoverTodos))
	}
}

// TestApplyDefects_DeterministicIDs: generated todo IDs must be stable
// functions of cycle + source position.
// Contract: "Generated identifiers must be stable functions of cycle,
// source position, and normalized content."
func TestApplyDefects_DeterministicIDs(t *testing.T) {
	record := core.FailedRecord{
		Cycle:   6,
		Verdict: "FAIL",
		Defects: []string{"deterministic-defect"},
	}
	s1 := &core.State{}
	core.ApplyDefectsAsCarryoverTodos(s1, record)
	s2 := &core.State{}
	core.ApplyDefectsAsCarryoverTodos(s2, record)

	if len(s1.CarryoverTodos) == 0 || len(s2.CarryoverTodos) == 0 {
		t.Fatal("expected todos in both states")
	}
	if s1.CarryoverTodos[0].ID != s2.CarryoverTodos[0].ID {
		t.Errorf("todo ID not deterministic: first=%q second=%q",
			s1.CarryoverTodos[0].ID, s2.CarryoverTodos[0].ID)
	}
}

// TestApplyDefects_PriorityIsP0: per-defect todos must carry P0 priority.
// Contract: "priority is P0".
func TestApplyDefects_PriorityIsP0(t *testing.T) {
	state := &core.State{}
	record := core.FailedRecord{
		Cycle:   7,
		Verdict: "FAIL",
		Defects: []string{"p0-priority defect"},
	}
	core.ApplyDefectsAsCarryoverTodos(state, record)
	for _, todo := range state.CarryoverTodos {
		if !strings.Contains(todo.Action, "p0-priority defect") {
			continue
		}
		if todo.Priority != "P0" {
			t.Errorf("per-defect todo Priority: got %q, want P0", todo.Priority)
		}
	}
	if len(state.CarryoverTodos) == 0 {
		t.Error("expected at least one todo")
	}
}

// TestApplyDefects_CyclesUnpickedStartsAtZero: CyclesUnpicked must start at 0.
// Contract: "CyclesUnpicked starts at zero".
func TestApplyDefects_CyclesUnpickedStartsAtZero(t *testing.T) {
	state := &core.State{}
	record := core.FailedRecord{
		Cycle:   8,
		Verdict: "FAIL",
		Defects: []string{"fresh unpicked defect"},
	}
	core.ApplyDefectsAsCarryoverTodos(state, record)
	for _, todo := range state.CarryoverTodos {
		if !strings.Contains(todo.Action, "fresh unpicked defect") {
			continue
		}
		if todo.CyclesUnpicked != 0 {
			t.Errorf("new todo CyclesUnpicked: got %d, want 0", todo.CyclesUnpicked)
		}
	}
	if len(state.CarryoverTodos) == 0 {
		t.Error("expected at least one todo")
	}
}

// TestApplyDefects_FirstSeenCycleMatchesRecord: FirstSeenCycle must equal
// the FailedRecord.Cycle.
// Contract: "FirstSeenCycle is the failed cycle".
func TestApplyDefects_FirstSeenCycleMatchesRecord(t *testing.T) {
	const cycle = 9
	state := &core.State{}
	record := core.FailedRecord{
		Cycle:   cycle,
		Verdict: "FAIL",
		Defects: []string{"first-seen-cycle defect"},
	}
	core.ApplyDefectsAsCarryoverTodos(state, record)
	for _, todo := range state.CarryoverTodos {
		if !strings.Contains(todo.Action, "first-seen-cycle defect") {
			continue
		}
		if todo.FirstSeenCycle != cycle {
			t.Errorf("FirstSeenCycle: got %d, want %d", todo.FirstSeenCycle, cycle)
		}
	}
	if len(state.CarryoverTodos) == 0 {
		t.Error("expected at least one todo")
	}
}

// TestApplyDefects_EachDefectGetsOwnTodo: three distinct defects must produce
// three separate todos with distinct IDs (not collapsed into one).
func TestApplyDefects_EachDefectGetsOwnTodo(t *testing.T) {
	state := &core.State{}
	defects := []string{"defect alpha", "defect beta", "defect gamma"}
	record := core.FailedRecord{
		Cycle:   10,
		Verdict: "FAIL",
		Defects: defects,
	}
	core.ApplyDefectsAsCarryoverTodos(state, record)
	if len(state.CarryoverTodos) < len(defects) {
		t.Errorf("want >= %d todos (one per defect), got %d", len(defects), len(state.CarryoverTodos))
	}
	// IDs must be distinct.
	ids := make(map[string]bool)
	for _, todo := range state.CarryoverTodos {
		if ids[todo.ID] {
			t.Errorf("duplicate todo ID %q", todo.ID)
		}
		ids[todo.ID] = true
	}
}

// TestApplyDefects_NilDefectsSlice: nil Defects slice must produce zero todos
// (same semantics as empty slice).
func TestApplyDefects_NilDefectsSlice(t *testing.T) {
	state := &core.State{}
	record := core.FailedRecord{
		Cycle:   11,
		Verdict: "FAIL",
		Defects: nil,
	}
	core.ApplyDefectsAsCarryoverTodos(state, record)
	if len(state.CarryoverTodos) != 0 {
		t.Errorf("nil Defects: want 0 per-defect todos, got %d", len(state.CarryoverTodos))
	}
}

// TestApplyDefects_ExistingTodosNotReplaced: pre-existing todos in state must
// be preserved after applying defects (defect application is additive).
func TestApplyDefects_ExistingTodosNotReplaced(t *testing.T) {
	state := &core.State{
		CarryoverTodos: []core.CarryoverTodo{
			{ID: "pre-existing", Action: "existing work", Priority: "P1"},
		},
	}
	record := core.FailedRecord{
		Cycle:   12,
		Verdict: "FAIL",
		Defects: []string{"new defect"},
	}
	core.ApplyDefectsAsCarryoverTodos(state, record)
	// Pre-existing todo must still be present.
	found := false
	for _, todo := range state.CarryoverTodos {
		if todo.ID == "pre-existing" {
			found = true
			break
		}
	}
	if !found {
		t.Error("pre-existing todos must not be removed by ApplyDefectsAsCarryoverTodos")
	}
}
