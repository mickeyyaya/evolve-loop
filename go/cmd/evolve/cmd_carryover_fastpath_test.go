package main

import "testing"

func TestCarryoverNoOpFastPath_ReturnsNoOpWhenNoMatchingID(t *testing.T) {
	statePath := writeFixtureState(t, "todo-a", "todo-b")
	remove := map[string]string{"todo-z": "drop"}

	res, ok := carryoverNoOpFastPath(statePath, remove)
	if !ok {
		t.Fatalf("carryoverNoOpFastPath returned ok=false when no removal id is present in state — the fast path must trigger on this input")
	}
	if res.Before != 2 || res.After != 2 {
		t.Errorf("fast-path no-op result = %+v, want Before=2 After=2 (no mutation)", res)
	}
	if res.Dropped != 0 || res.Clustered != 0 {
		t.Errorf("fast-path no-op result = %+v, want Dropped=0 Clustered=0", res)
	}
}

func TestCarryoverNoOpFastPath_FallsThroughWhenMatchingIDPresent(t *testing.T) {
	statePath := writeFixtureState(t, "todo-a", "todo-drop-me")
	remove := map[string]string{"todo-drop-me": "drop"}

	_, ok := carryoverNoOpFastPath(statePath, remove)
	if ok {
		t.Fatalf("carryoverNoOpFastPath returned ok=true when a removal id IS present — it must fall through to the locked update instead of short-circuiting")
	}
}

func TestCarryoverNoOpFastPath_FallsThroughOnUnreadableState(t *testing.T) {
	remove := map[string]string{"todo-z": "drop"}

	_, ok := carryoverNoOpFastPath("/nonexistent/path/state.json", remove)
	if ok {
		t.Fatalf("carryoverNoOpFastPath returned ok=true for an unreadable state path — an unlocked pre-read error must fall through to the locked update, never short-circuit on a guess")
	}
}
