package carryover

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// CapRunes truncates s to maxRunes with a trailing ellipsis rune.
func CapRunes(s string, maxRunes int) string {
	if r := []rune(s); len(r) > maxRunes {
		return string(r[:maxRunes]) + "…"
	}
	return s
}

// Summary is the failure-born todo's action and the FailedRecord's summary:
// the message capped at MaxSummaryRunes with the truncation marker (a
// different rule from CapRunes' ellipsis — both preserved).
func Summary(cycle int, phase cyclestate.Phase, err error) string {
	msg := err.Error()
	r := []rune(msg)
	if len(r) > MaxSummaryRunes {
		msg = string(r[:MaxSummaryRunes]) + " ...[truncated]"
	}
	return fmt.Sprintf("cycle %d failed during %s: %s", cycle, phase, msg)
}

// Append admits one todo through the ONE admission rule: a cross-cycle
// fingerprint twin only refreshes the survivor's expiry; an id twin is
// skipped; otherwise the todo appends last. A nil state is inert.
func (l *Lifecycle) Append(state *cyclestate.State, todo cyclestate.CarryoverTodo) {
	if state == nil {
		return
	}
	l.admit(state, todo)
}

// ApplyDefects mints one P0 todo per non-blank defect of a failed record
// (ids count non-blank defects only; the action is capped; the record's
// expiry is inherited, never recomputed) through the same admission rule.
func (l *Lifecycle) ApplyDefects(state *cyclestate.State, record cyclestate.FailedRecord) {
	n := 0
	for _, defect := range record.Defects {
		if strings.TrimSpace(defect) == "" {
			continue
		}
		id := fmt.Sprintf("cycle-%d-defect-%d", record.Cycle, n)
		n++
		l.admit(state, cyclestate.CarryoverTodo{
			ID:             id,
			Action:         "Fix defect from cycle " + strconv.Itoa(record.Cycle) + ": " + CapRunes(defect, MaxActionRunes),
			Priority:       PriorityBlocking,
			FirstSeenCycle: record.Cycle,
			CyclesUnpicked: 0,
			ExpiresAt:      record.ExpiresAt,
		})
	}
}

// admit is the one dedupe rule both mint paths used to spell separately.
func (l *Lifecycle) admit(state *cyclestate.State, todo cyclestate.CarryoverTodo) {
	if idx := fingerprintIndex(state.CarryoverTodos, todo.Action); idx >= 0 {
		refreshExpiry(&state.CarryoverTodos[idx], todo.ExpiresAt)
		return
	}
	if HasID(state.CarryoverTodos, todo.ID) {
		return
	}
	state.CarryoverTodos = append(state.CarryoverTodos, todo)
}
