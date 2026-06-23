package core_test

// RED note: this file references core.ApplyDefectsAsCarryoverTodos which does
// not exist yet — the compile error is the intended RED signal. Builder adds
// the function in go/internal/core/failure_learning.go (D2-e slice).

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// TestDefects_BecomeCarryoverTodos verifies that each entry in
// FailedRecord.Defects becomes its own CarryoverTodo (not just one generic
// phase-failed todo). A single generic todo does NOT meet the D2 contract.
func TestDefects_BecomeCarryoverTodos(t *testing.T) {
	defects := []string{
		"unbounded fan-out in auditor verify path",
		"nil pointer in router when signals absent",
	}
	state := &core.State{}
	record := core.FailedRecord{
		Cycle:          1,
		Verdict:        "FAIL",
		Classification: "test-defects",
		Defects:        defects,
		Summary:        "two distinct defects",
	}
	core.ApplyDefectsAsCarryoverTodos(state, record)

	if len(state.CarryoverTodos) < len(defects) {
		t.Errorf("want >= %d CarryoverTodos (one per defect), got %d; a single generic todo is insufficient",
			len(defects), len(state.CarryoverTodos))
	}
	for i, defect := range defects {
		found := false
		for _, todo := range state.CarryoverTodos {
			if strings.Contains(todo.Action, defect) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("defect[%d] %q has no corresponding CarryoverTodo action", i, defect)
		}
	}
}

// TestDefects_BecomeCarryoverTodos_NegativeEmptyDefects verifies that an empty
// Defects slice results in zero new per-defect todos (boundary / OOD case).
func TestDefects_BecomeCarryoverTodos_NegativeEmptyDefects(t *testing.T) {
	state := &core.State{}
	record := core.FailedRecord{
		Cycle:   2,
		Verdict: "FAIL",
		Defects: nil,
		Summary: "no individual defects",
	}
	core.ApplyDefectsAsCarryoverTodos(state, record)
	if len(state.CarryoverTodos) != 0 {
		t.Errorf("empty Defects: want 0 CarryoverTodos added, got %d", len(state.CarryoverTodos))
	}
}
