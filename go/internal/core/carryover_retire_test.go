package core

// carryover_retire_test.go — RED contract for cycle-1440 task
// `carryover-pass-retirement`.
//
// Defect: mergeCarryoverTodos (failure_learning.go) unions disk+incoming and
// dedupes by ID, but NOTHING ever removes an entry. A carryover todo whose work
// actually shipped therefore persists forever, saturating the router prompt's
// 20-slot carryover window with already-done work (the 2026-08-10 investigation
// found 124 of 254 live entries were stale duplicates of a few classes).
//
// Contract under test (not yet implemented — these tests MUST fail RED until
// Builder adds the PASS-closeout deletion path):
//
//	RetireCarryoverTodos(todos, committedIDs) []CarryoverTodo
//
// retires (a) every entry whose ID is in the committed set, and (b) every entry
// sharing a retired entry's cross-cycle Action fingerprint
// (carryoverActionFingerprint) — the per-cycle re-mints of the SAME class that
// the ID-keyed dedupe never collapsed. Everything else survives untouched, in
// order.

import (
	"reflect"
	"testing"
)

func retireIDs(todos []CarryoverTodo) []string {
	out := make([]string, 0, len(todos))
	for _, t := range todos {
		out = append(out, t.ID)
	}
	return out
}

// TestRetireCarryoverTodos_CommittedIDRetires is the primary case: the id the
// cycle actually committed leaves the array; the unrelated entry stays.
func TestRetireCarryoverTodos_CommittedIDRetires(t *testing.T) {
	todos := []CarryoverTodo{
		{ID: "carryover-pass-retirement", Action: "add PASS-closeout deletion path", Priority: "HIGH", FirstSeenCycle: 1421},
		{ID: "unrelated-item", Action: "something else entirely", Priority: "MEDIUM", FirstSeenCycle: 1430},
	}
	got := RetireCarryoverTodos(todos, []string{"carryover-pass-retirement"})
	if want := []string{"unrelated-item"}; !reflect.DeepEqual(retireIDs(got), want) {
		t.Errorf("RetireCarryoverTodos ids = %v, want %v — a committed id must not survive PASS closeout", retireIDs(got), want)
	}
}

// TestRetireCarryoverTodos_FingerprintVariantRetires pins the second half of
// the rule: the same failure class re-minted on a later cycle carries a
// DIFFERENT id but the same normalized Action, and must retire with its twin.
// Without this, retirement leaks exactly the duplicates the fingerprint index
// was built to collapse.
func TestRetireCarryoverTodos_FingerprintVariantRetires(t *testing.T) {
	todos := []CarryoverTodo{
		{ID: "committed-id", Action: "Fix the stage refusal router in cycle 1421", FirstSeenCycle: 1421},
		{ID: "scout-minted-variant", Action: "fix the stage refusal router in cycle-1435", FirstSeenCycle: 1435},
		{ID: "different-class", Action: "normalize fingerprint path variance", FirstSeenCycle: 1436},
	}
	got := RetireCarryoverTodos(todos, []string{"committed-id"})
	if want := []string{"different-class"}; !reflect.DeepEqual(retireIDs(got), want) {
		t.Errorf("RetireCarryoverTodos ids = %v, want %v — a cycle-token variant of a retired entry's Action is the SAME class and must retire with it",
			retireIDs(got), want)
	}
}

// TestRetireCarryoverTodos_UnmatchedSurvivesInOrder is the negative case: an id
// nobody committed must be untouched, and surviving order must be stable (the
// router window is ordered).
func TestRetireCarryoverTodos_UnmatchedSurvivesInOrder(t *testing.T) {
	todos := []CarryoverTodo{
		{ID: "a", Action: "alpha"},
		{ID: "b", Action: "bravo"},
		{ID: "c", Action: "charlie"},
	}
	got := RetireCarryoverTodos(todos, []string{"never-committed"})
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(retireIDs(got), want) {
		t.Errorf("RetireCarryoverTodos ids = %v, want %v — an uncommitted id must never be retired", retireIDs(got), want)
	}
}

// TestRetireCarryoverTodos_EdgeInputs covers the empty/nil/blank boundary: no
// committed ids retires nothing, and a blank id must not match a blank-id entry
// (a malformed entry is not "committed").
func TestRetireCarryoverTodos_EdgeInputs(t *testing.T) {
	todos := []CarryoverTodo{{ID: "keep-me", Action: "work"}, {ID: "", Action: "malformed"}}

	if got := RetireCarryoverTodos(todos, nil); len(got) != 2 {
		t.Errorf("nil committed set retired %d entr(ies), want 0 retired", 2-len(got))
	}
	if got := RetireCarryoverTodos(todos, []string{}); len(got) != 2 {
		t.Errorf("empty committed set retired %d entr(ies), want 0 retired", 2-len(got))
	}
	if got := RetireCarryoverTodos(todos, []string{"  "}); len(got) != 2 {
		t.Errorf("blank committed id retired %d entr(ies), want 0 retired", 2-len(got))
	}
	if got := RetireCarryoverTodos(nil, []string{"keep-me"}); len(got) != 0 {
		t.Errorf("nil todos returned %d entr(ies), want 0", len(got))
	}
}

// TestRetireCarryoverTodos_DoesNotMutateInput pins immutability (core rule:
// return new slices, never mutate in place) — the caller re-reads state under a
// lock and a mutated input would corrupt a concurrent peer's merge.
func TestRetireCarryoverTodos_DoesNotMutateInput(t *testing.T) {
	todos := []CarryoverTodo{{ID: "gone", Action: "x"}, {ID: "stays", Action: "y"}}
	_ = RetireCarryoverTodos(todos, []string{"gone"})
	if len(todos) != 2 || todos[0].ID != "gone" || todos[1].ID != "stays" {
		t.Errorf("input slice was mutated: %v", retireIDs(todos))
	}
}
