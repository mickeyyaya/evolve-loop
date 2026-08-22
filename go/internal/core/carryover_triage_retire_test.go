package core

// carryover_triage_retire_test.go — cover the edges the shipped cycle-1538
// reproduction did not. That test pins only the happy path (a dropped id is
// retired, an unrelated one survives); these pin the cases where retiring too
// EAGERLY would silently lose live work, which is the worse failure.

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTriageDecisionFile(t *testing.T, body string) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write decision: %v", err)
	}
	return ws
}

func retireTestTodos(ids ...string) []CarryoverTodo {
	out := make([]CarryoverTodo, 0, len(ids))
	for _, id := range ids {
		out = append(out, CarryoverTodo{ID: id, Action: "work " + id})
	}
	return out
}

func retireTestIDs(todos []CarryoverTodo) []string {
	out := make([]string, 0, len(todos))
	for _, t := range todos {
		out = append(out, t.ID)
	}
	return out
}

// DEFERRED is triage saying "not this cycle", not "never" — retiring it would
// silently delete work triage explicitly intended to keep.
func TestRetireTriageDropped_DeferredSurvives(t *testing.T) {
	ws := writeTriageDecisionFile(t, `{"cycle":1,"top_n":[],"deferred":[{"id":"later"}],"dropped":[{"id":"stale"}]}`)
	state := &State{CarryoverTodos: retireTestTodos("stale", "later", "live")}

	retireTriageDroppedCarryover(state, ws)

	if got := retireTestIDs(state.CarryoverTodos); len(got) != 2 || got[0] != "later" || got[1] != "live" {
		t.Fatalf("only the DROPPED id may be retired; deferred and live must survive. got %v", got)
	}
}

// Every ambiguous input keeps every todo: forgetting live work is worse than
// carrying a stale entry one more cycle.
func TestRetireTriageDropped_AmbiguousInputsRetireNothing(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"malformed json", `{not json`},
		{"no dropped key", `{"cycle":1,"top_n":[{"id":"a"}]}`},
		{"empty dropped", `{"cycle":1,"dropped":[]}`},
		{"blank id", `{"cycle":1,"dropped":[{"id":"   "}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := &State{CarryoverTodos: retireTestTodos("keep-me", "and-me")}
			retireTriageDroppedCarryover(state, writeTriageDecisionFile(t, tc.body))
			if got := retireTestIDs(state.CarryoverTodos); len(got) != 2 {
				t.Fatalf("%s must retire nothing; got %v", tc.name, got)
			}
		})
	}
}

// A continuation/lane cycle carries no triage-decision.json at all.
func TestRetireTriageDropped_NoDecisionFileRetiresNothing(t *testing.T) {
	state := &State{CarryoverTodos: retireTestTodos("keep-me")}
	retireTriageDroppedCarryover(state, t.TempDir())
	if len(state.CarryoverTodos) != 1 {
		t.Fatalf("a cycle with no triage decision must retire nothing; got %v", retireTestIDs(state.CarryoverTodos))
	}
}

// Defensive inputs must not panic.
func TestRetireTriageDropped_NilAndEmptyAreInert(t *testing.T) {
	retireTriageDroppedCarryover(nil, "anywhere")
	state := &State{}
	retireTriageDroppedCarryover(state, "")
	if len(state.CarryoverTodos) != 0 {
		t.Fatalf("expected no todos")
	}
}

// Several dropped ids retire together, and order of the survivors is preserved
// so the next planner sees a stable list.
func TestRetireTriageDropped_MultipleDroppedPreserveSurvivorOrder(t *testing.T) {
	ws := writeTriageDecisionFile(t, `{"cycle":1,"dropped":[{"id":"b"},{"id":"d"}]}`)
	state := &State{CarryoverTodos: retireTestTodos("a", "b", "c", "d", "e")}

	retireTriageDroppedCarryover(state, ws)

	got := retireTestIDs(state.CarryoverTodos)
	if len(got) != 3 || got[0] != "a" || got[1] != "c" || got[2] != "e" {
		t.Fatalf("survivors must keep their order; got %v", got)
	}
}
