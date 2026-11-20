package carryover

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func documentedOnMain(ids ...string) func(string) bool {
	on := make(map[string]bool, len(ids))
	for _, id := range ids {
		on[id] = true
	}
	return func(id string) bool { return on[id] }
}

func assertTodoIDs(t *testing.T, todos []cyclestate.CarryoverTodo, msg string, want ...string) {
	t.Helper()
	if len(todos) != len(want) {
		t.Fatalf("%s: got %+v, want %v", msg, todos, want)
	}
	for i, w := range want {
		if todos[i].ID != w {
			t.Fatalf("%s: index %d got %q, want %q (%+v)", msg, i, todos[i].ID, w, todos)
		}
	}
}

func TestJudgmentRetire_ATodoWithNoDocumentStays(t *testing.T) {
	st := &cyclestate.State{CarryoverTodos: []cyclestate.CarryoverTodo{
		{ID: "cycle-5-judgment-plan-review", Action: "the plan widens the lane scope"},
	}}

	New().RetireDocumentedJudgments(st, documentedOnMain())

	assertTodoIDs(t, st.CarryoverTodos, "a judgment todo with no document on main must stay", "cycle-5-judgment-plan-review")
}

func TestJudgmentRetire_ATodoWithItsDocumentLeavesAndTheOthersKeepTheirOrder(t *testing.T) {
	st := &cyclestate.State{CarryoverTodos: []cyclestate.CarryoverTodo{
		{ID: "cycle-5-judgment-plan-review", Action: "plan lesson"},
		{ID: "cycle-6-judgment-audit", Action: "audit lesson"},
		{ID: "cycle-7-judgment-build", Action: "build lesson"},
	}}

	New().RetireDocumentedJudgments(st, documentedOnMain("cycle-6-judgment-audit"))

	assertTodoIDs(t, st.CarryoverTodos, "only the todo with its document leaves, the rest keep their order", "cycle-5-judgment-plan-review", "cycle-7-judgment-build")
}

func TestJudgmentRetire_ASecondRunChangesNothing(t *testing.T) {
	st := &cyclestate.State{CarryoverTodos: []cyclestate.CarryoverTodo{
		{ID: "cycle-5-judgment-plan-review", Action: "plan lesson", ExpiresAt: "2026-12-01T00:00:00Z"},
		{ID: "cycle-6-judgment-audit", Action: "audit lesson"},
	}}
	documented := documentedOnMain("cycle-6-judgment-audit")

	New().RetireDocumentedJudgments(st, documented)
	afterFirst := append([]cyclestate.CarryoverTodo(nil), st.CarryoverTodos...)
	New().RetireDocumentedJudgments(st, documented)

	if len(st.CarryoverTodos) != len(afterFirst) || st.CarryoverTodos[0] != afterFirst[0] {
		t.Fatalf("a second run must change nothing: before %+v, after %+v", afterFirst, st.CarryoverTodos)
	}
	if len(st.CarryoverTodos) != 1 || st.CarryoverTodos[0].ExpiresAt != "2026-12-01T00:00:00Z" {
		t.Fatalf("the surviving todo must keep every field: %+v", st.CarryoverTodos)
	}
}

func TestJudgmentRetire_ATodoThatIsNotAJudgmentIsNeverAskedAboutAndNeverLeaves(t *testing.T) {
	st := &cyclestate.State{CarryoverTodos: []cyclestate.CarryoverTodo{
		{ID: "cycle-5-failed-build", Action: "cycle 5 failed during build: a"},
		{ID: "plain-id", Action: "plain action"},
		{ID: "cycle-x-judgment-plan", Action: "a cycle that is no number"},
		{ID: "cycle-8-judgment-Plan", Action: "a phase with a capital"},
	}}
	var asked []string
	everythingDocumented := func(id string) bool {
		asked = append(asked, id)
		return true
	}

	New().RetireDocumentedJudgments(st, everythingDocumented)

	if len(asked) != 0 {
		t.Fatalf("the document check must run only for an id of the form cycle-<N>-judgment-<phase>: asked %v", asked)
	}
	assertTodoIDs(t, st.CarryoverTodos, "a todo that is no judgment lesson must stay", "cycle-5-failed-build", "plain-id", "cycle-x-judgment-plan", "cycle-8-judgment-Plan")
}

func TestJudgmentRetire_ANilStateAndAnEmptyLogAreInert(t *testing.T) {
	New().RetireDocumentedJudgments(nil, documentedOnMain())
	st := &cyclestate.State{}

	New().RetireDocumentedJudgments(st, documentedOnMain("cycle-5-judgment-plan-review"))

	if len(st.CarryoverTodos) != 0 {
		t.Fatalf("an empty log stays empty: %+v", st.CarryoverTodos)
	}
}
