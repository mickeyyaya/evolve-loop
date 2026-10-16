package carryover

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/textcap"
)

func CapRunes(s string, maxRunes int) string { return textcap.CapRunes(s, maxRunes) }

func TruncateRunes(s string, max int) string { return textcap.TruncateRunes(s, max) }

func Summary(cycle int, phase cyclestate.Phase, err error) string {
	msg := cyclestate.WithoutDetail(err.Error())
	r := []rune(msg)
	if len(r) > MaxSummaryRunes {
		msg = string(r[:MaxSummaryRunes]) + " ...[truncated]"
	}
	return fmt.Sprintf("cycle %d failed during %s: %s", cycle, phase, msg)
}

func (l *Lifecycle) Append(state *cyclestate.State, todo cyclestate.CarryoverTodo) {
	if state == nil {
		return
	}
	l.admit(state, todo)
}

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
