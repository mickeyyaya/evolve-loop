package carryover

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// MergeTodos unions the on-disk and incoming todos by id, disk first, the
// DISK copy kept on a shared id (a peer's concurrent todo is never clobbered;
// an in-memory refresh of an already-persisted todo does not land — the
// preserved quirk Q3). Returns a new slice.
func MergeTodos(disk, incoming []cyclestate.CarryoverTodo) []cyclestate.CarryoverTodo {
	out := append([]cyclestate.CarryoverTodo(nil), disk...)
	for _, td := range incoming {
		if !HasID(out, td.ID) {
			out = append(out, td)
		}
	}
	return out
}

// MergeFailedRecords unions the on-disk and incoming records by the key
// cycle+ts+verdict+recordedAt, disk order first, incoming winning on a shared
// key. Returns a new slice.
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
		add(r) // incoming overrides disk for a shared key
	}
	out := make([]cyclestate.FailedRecord, 0, len(order))
	for _, k := range order {
		out = append(out, byKey[k])
	}
	return out
}
