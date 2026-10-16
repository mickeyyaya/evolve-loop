package carryover

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func Retire(todos []cyclestate.CarryoverTodo, committedIDs []string) []cyclestate.CarryoverTodo {
	committed := make(map[string]bool, len(committedIDs))
	for _, id := range committedIDs {
		if id = strings.TrimSpace(id); id != "" {
			committed[id] = true
		}
	}
	if len(committed) == 0 || len(todos) == 0 {
		return append([]cyclestate.CarryoverTodo(nil), todos...)
	}
	retiredFP := map[string]bool{}
	for _, t := range todos {
		if committed[t.ID] {
			if fp := fingerprint(t.Action); fp != "" {
				retiredFP[fp] = true
			}
		}
	}
	out := make([]cyclestate.CarryoverTodo, 0, len(todos))
	for _, t := range todos {
		if committed[t.ID] || retiredFP[fingerprint(t.Action)] {
			continue
		}
		out = append(out, t)
	}
	return out
}
