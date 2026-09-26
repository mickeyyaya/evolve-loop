package core

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

func TestRetireCarryoverTodos_DoesNotMutateInput(t *testing.T) {
	todos := []CarryoverTodo{{ID: "gone", Action: "x"}, {ID: "stays", Action: "y"}}
	_ = RetireCarryoverTodos(todos, []string{"gone"})
	if len(todos) != 2 || todos[0].ID != "gone" || todos[1].ID != "stays" {
		t.Errorf("input slice was mutated: %v", retireIDs(todos))
	}
}
