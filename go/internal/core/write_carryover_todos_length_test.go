package core

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// maxRenderedOversizedTodo bounds what a correct per-item cap must produce: a
// single todo with a 4000-rune Action renders to ~4130 bytes uncapped, and the
// bound is loose enough for any reasonable per-item cap yet tight enough that
// an uncapped passthrough still fails it.
const maxRenderedOversizedTodo = 1000

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

func TestWriteCarryoverTodos_EmptyOmitsSection(t *testing.T) {
	var b strings.Builder
	writeCarryoverTodos(&b, nil)
	if b.String() != "" {
		t.Errorf("empty todos must omit the section entirely; got %q", b.String())
	}
}

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
