package advisor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/textcap"
)

// MaxCarryoverTodosInPrompt caps how many carryover todos render into the prompt.
const MaxCarryoverTodosInPrompt = 20

// maxCarryoverTodoActionRunes also bounds oversized entries already on disk, which creation-time caps cannot shrink.
const maxCarryoverTodoActionRunes = 600

// carryoverPriorityRank ranks an unknown priority 0, so it sorts last without being dropped.
func carryoverPriorityRank(p string) int {
	switch strings.ToUpper(strings.TrimSpace(p)) {
	case "P0":
		return 6
	case "P1", "H", "HIGH":
		return 5
	case "P2":
		return 4
	case "P3", "M", "MED", "MEDIUM":
		return 3
	case "L", "LOW":
		return 1
	default:
		return 0
	}
}

// WriteCarryoverTodos renders the carryover todos, highest priority and most recent first, capped in count and length.
func WriteCarryoverTodos(b *strings.Builder, todos []router.CarryoverTodo) {
	if len(todos) == 0 {
		return
	}
	b.WriteString("\n## Carryover todos from previous cycles (consider when selecting phases)\n")
	// A stable sort of a copy: ties keep on-disk order and the caller's slice is untouched.
	ordered := append([]router.CarryoverTodo(nil), todos...)
	sort.SliceStable(ordered, func(i, j int) bool {
		ri, rj := carryoverPriorityRank(ordered[i].Priority), carryoverPriorityRank(ordered[j].Priority)
		if ri != rj {
			return ri > rj
		}
		return ordered[i].FirstSeenCycle > ordered[j].FirstSeenCycle
	})
	limit := min(len(ordered), MaxCarryoverTodosInPrompt)
	for i := 0; i < limit; i++ {
		t := ordered[i]
		fmt.Fprintf(b, "- [%s] %s: %s (first_seen_cycle=%d, cycles_unpicked=%d)\n",
			t.Priority, t.ID, textcap.CapRunes(t.Action, maxCarryoverTodoActionRunes), t.FirstSeenCycle, t.CyclesUnpicked)
	}
	if len(ordered) > limit {
		fmt.Fprintf(b, "- ... %d more carryover todo(s) omitted from prompt\n", len(ordered)-limit)
	}
}
