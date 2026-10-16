package carryover

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func MergeTodos(disk, incoming []cyclestate.CarryoverTodo) []cyclestate.CarryoverTodo {
	out := append([]cyclestate.CarryoverTodo(nil), disk...)
	for _, td := range incoming {
		if !HasID(out, td.ID) {
			out = append(out, td)
		}
	}
	return out
}

func MergeFailedRecords(disk, incoming []cyclestate.FailedRecord) []cyclestate.FailedRecord {
	key := func(r cyclestate.FailedRecord) string {
		return fmt.Sprintf("%d\x00%s\x00%s\x00%s", r.Cycle, r.TS, r.Verdict, r.RecordedAt)
	}
	byKey := make(map[string]cyclestate.FailedRecord, len(disk)+len(incoming))
	order := make([]string, 0, len(disk)+len(incoming))
	add := func(r cyclestate.FailedRecord) {
		k := key(r)
		if _, seen := byKey[k]; !seen {
			order = append(order, k)
		}
		byKey[k] = r
	}
	for _, r := range disk {
		add(r)
	}
	for _, r := range incoming {
		add(r)
	}
	out := make([]cyclestate.FailedRecord, 0, len(order))
	for _, k := range order {
		out = append(out, byKey[k])
	}
	return out
}
