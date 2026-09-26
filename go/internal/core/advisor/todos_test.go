package advisor

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestCarryoverPriorityRank_BindsToTheCarryoverVocabulary(t *testing.T) {
	for p, want := range map[string]int{
		"P0": 6, "P1": 5, "H": 5, "HIGH": 5, "P2": 4, "P3": 3, "M": 3, "MED": 3, "MEDIUM": 3, "L": 1, "LOW": 1,
		" p1 ": 5, "high": 5, "blocking": 0, "": 0, "P9": 0,
	} {
		if got := carryoverPriorityRank(p); got != want {
			t.Errorf("carryoverPriorityRank(%q) = %d, want %d", p, got, want)
		}
	}
	if carryoverPriorityRank(carryover.PriorityBlocking) != 6 || carryoverPriorityRank(carryover.PriorityLesson) != 5 ||
		carryoverPriorityRank(carryover.PriorityPrescription) != 5 || carryoverPriorityRank(carryover.PriorityMemoDefault) != 3 {
		t.Fatal("P0 > P1 == high > medium in the advisor's rank table (the unit-03 vocabulary)")
	}
}

func TestWriteCarryoverTodos_OrdersByRankThenRecencyCapsAtTwentyAndSixHundredRunes(t *testing.T) {
	todos := richCarryoverTodos()
	before := append([]router.CarryoverTodo(nil), todos...)
	var b strings.Builder
	WriteCarryoverTodos(&b, todos)
	out := b.String()
	if strings.Count(out, "- [") != MaxCarryoverTodosInPrompt || !strings.Contains(out, "- ... 3 more carryover todo(s) omitted from prompt\n") {
		t.Errorf("exactly %d lines and the trailer:\n%s", MaxCarryoverTodosInPrompt, out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")[1:]
	if !strings.HasPrefix(lines[0], "- [P0] cycle-40-todo-00:") || !strings.HasPrefix(lines[1], "- [P0] cycle-39-todo-21:") || !strings.HasPrefix(lines[2], "- [P0] cycle-38-todo-17:") || !strings.HasPrefix(lines[3], "- [p0] cycle-36-todo-14:") {
		t.Errorf("rank desc, then recency desc:\n%s", strings.Join(lines[:4], "\n"))
	}
	if !strings.HasPrefix(lines[4], "- [H] cycle-40-todo-20:") || !strings.HasPrefix(lines[5], "- [P1] cycle-39-todo-01:") || !strings.HasPrefix(lines[6], "- [P1] cycle-39-todo-16:") {
		t.Errorf("the rank-5 block follows, most recent first, equal cycles in on-disk order:\n%s", strings.Join(lines[4:8], "\n"))
	}
	if !strings.Contains(out, strings.Repeat("é", 600)+"…") || strings.Contains(out, strings.Repeat("é", 601)) {
		t.Errorf("a 700-rune action renders as 600 runes + the ellipsis:\n%s", out)
	}
	for i := range todos {
		if todos[i] != before[i] {
			t.Fatal("the caller's slice must never be mutated")
		}
	}
	var none strings.Builder
	WriteCarryoverTodos(&none, nil)
	if none.Len() != 0 {
		t.Errorf("no todos render nothing: %q", none.String())
	}
	var atCap strings.Builder
	WriteCarryoverTodos(&atCap, todos[:MaxCarryoverTodosInPrompt])
	if strings.Contains(atCap.String(), "omitted from prompt") {
		t.Error("exactly the cap renders no trailer")
	}
	var junk strings.Builder
	WriteCarryoverTodos(&junk, []router.CarryoverTodo{{ID: "a", Priority: "\x00weird", Action: "x"}, {ID: "b", Priority: "P0", Action: "y", FirstSeenCycle: 1, CyclesUnpicked: 2}})
	if !strings.HasPrefix(junk.String(), "\n## Carryover todos from previous cycles (consider when selecting phases)\n- [P0] b: y (first_seen_cycle=1, cycles_unpicked=2)\n- [\x00weird] a: x") {
		t.Errorf("a malformed priority sorts last without panicking: %q", junk.String())
	}
	if MaxCarryoverTodosInPrompt != 20 {
		t.Errorf("MaxCarryoverTodosInPrompt = %d, want 20", MaxCarryoverTodosInPrompt)
	}
}
