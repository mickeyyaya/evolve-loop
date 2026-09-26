package core_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

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
