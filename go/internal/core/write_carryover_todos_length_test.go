package core

// write_carryover_todos_length_test.go — cycle-488 RED tests for
// cap-carryover-todo-render-length (Task 2). White-box (`package core`) because
// writeCarryoverTodos is unexported.
//
// writeCarryoverTodos (phase_advisor.go) is the SOLE injection site for
// carryoverTodos into any agent prompt. It caps the COUNT at
// maxCarryoverTodosInPrompt (20) but not the per-item byte length, so an
// oversized stored Action — including the 54 entries already on disk today,
// which Task 1's creation-time fix cannot retroactively shrink — still renders
// in full. This is defense-in-depth: a render-time per-item cap protects the
// router prompt immediately and guards any future creation path.
//
// These tests MUST fail before the Builder adds the per-item render cap (the
// first is RED; the empty-omit and omitted-trailer tests are regression pins
// that are GREEN today and must stay GREEN through the edit).

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// maxRenderedOversizedTodo is the RED ceiling. A single todo carrying a
// 4000-rune Action renders to ~4130 bytes today (header + line + full Action).
// A correct per-item cap brings the whole section well under 1000 bytes. The
// bound is deliberately loose so the Builder picks the exact per-item cap, yet
// tight enough that a no-op (4000-rune passthrough) fails.
const maxRenderedOversizedTodo = 1000

// TestWriteCarryoverTodosCapsPerItemLength (Task2-AC: an oversized Action
// renders within a fixed byte bound). RED today: writeCarryoverTodos renders
// the full 4000-rune Action.
func TestWriteCarryoverTodosCapsPerItemLength(t *testing.T) {
	var b strings.Builder
	writeCarryoverTodos(&b, []router.CarryoverTodo{{
		ID:             "cycle-366-failed-ship",
		Action:         strings.Repeat("x", 4000),
		Priority:       "P0",
		FirstSeenCycle: 366,
		CyclesUnpicked: 0,
	}})
	out := b.String()
	if out == "" {
		t.Fatal("a single non-empty todo must render a section")
	}
	if n := len(out); n > maxRenderedOversizedTodo {
		t.Errorf("oversized Action rendered unbounded: section is %d bytes, want <= %d — writeCarryoverTodos must cap per-item Action length at render time",
			n, maxRenderedOversizedTodo)
	}
	// The todo must still be identifiable after truncation — capping length must
	// not erase the ID/priority the router needs to reason about it.
	if !strings.Contains(out, "cycle-366-failed-ship") {
		t.Errorf("per-item cap must preserve the todo ID; got %q", out)
	}
}

// TestWriteCarryoverTodos_EmptyOmitsSection (Task2-AC negative/regression):
// zero todos must render NOTHING (no header, no trailer). Pre-existing GREEN;
// pins the no-regression contract against a render-cap refactor.
func TestWriteCarryoverTodos_EmptyOmitsSection(t *testing.T) {
	var b strings.Builder
	writeCarryoverTodos(&b, nil)
	if b.String() != "" {
		t.Errorf("empty todos must omit the section entirely; got %q", b.String())
	}
}

// TestWriteCarryoverTodos_OmittedTrailerFires (Task2-AC boundary/regression):
// with more than maxCarryoverTodosInPrompt (20) todos, exactly 20 render and
// the "... N more ... omitted" trailer reports the remainder. Pre-existing
// GREEN; pins the count-cap behavior that must survive the per-item change.
func TestWriteCarryoverTodos_OmittedTrailerFires(t *testing.T) {
	const total = 25
	todos := make([]router.CarryoverTodo, total)
	for i := range todos {
		todos[i] = router.CarryoverTodo{
			ID:       "todo-" + strings.Repeat("a", 1), // short, distinct enough
			Action:   "short action",
			Priority: "P0",
		}
	}
	var b strings.Builder
	writeCarryoverTodos(&b, todos)
	out := b.String()
	if got := strings.Count(out, "- [P0]"); got != maxCarryoverTodosInPrompt {
		t.Errorf("rendered %d todo lines, want the count cap of %d", got, maxCarryoverTodosInPrompt)
	}
	if !strings.Contains(out, "5 more carryover todo(s) omitted") {
		t.Errorf("omitted-count trailer must report the %d remaining todos; got %q", total-maxCarryoverTodosInPrompt, out)
	}
}
