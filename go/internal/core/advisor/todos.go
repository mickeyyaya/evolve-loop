package advisor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/textcap"
)

// MaxCarryoverTodosInPrompt caps how many carryover todos render into the
// prompt. Core projects it for the by-name carryover tests.
const MaxCarryoverTodosInPrompt = 20

// maxCarryoverTodoActionRunes bounds each rendered todo Action at the sole
// prompt-injection site (defense-in-depth). It guards the oversized entries
// already on disk — which the creation-time caps in the failure-learning
// engine cannot retroactively shrink — plus any future creation path.
const maxCarryoverTodoActionRunes = 600

// carryoverPriorityRank maps a Priority string to a severity rank (higher =
// more severe). An unknown/malformed priority ranks lowest (0) so it sorts to
// the bottom without dropping the entry — the renderer stays total. The
// spellings are the carryover unit's vocabulary (P0 blocking, P1 lesson and
// prescription, P3 memo default) plus the legacy word forms.
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

// WriteCarryoverTodos renders the unresolved carryover todos, highest
// priority and most recent first, capped in count and per-item length.
func WriteCarryoverTodos(b *strings.Builder, todos []router.CarryoverTodo) {
	if len(todos) == 0 {
		return
	}
	b.WriteString("\n## Carryover todos from previous cycles (consider when selecting phases)\n")
	// When the array exceeds the count cap, render the HIGHEST-PRIORITY /
	// MOST-RECENT entries rather than a naive insertion-order (oldest-first)
	// prefix — the old todos[:20] silently hid the newest, most severe items
	// (e.g. cycle-505's leak) behind "N omitted". Sort a COPY (stable, so ties
	// keep on-disk order) — never mutate the caller's slice.
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
