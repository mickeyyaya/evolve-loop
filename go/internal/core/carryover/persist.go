package carryover

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

type Store interface {
	WriteState(ctx context.Context, s cyclestate.State) error
}

type Updater interface {
	UpdateState(ctx context.Context, mutate func(*cyclestate.State)) (cyclestate.State, error)
}

func (l *Lifecycle) Persist(ctx context.Context, store Store, state *cyclestate.State) {
	if state == nil {
		return
	}
	cycle := state.LastCycleNumber
	u, ok := store.(Updater)
	if !ok {
		if err := store.WriteState(ctx, *state); err != nil {
			l.warn("Lifecycle.Persist", cycle, CodePersistFailed, "state write failed: "+err.Error(), map[string]string{"step": "write"})
		}
		return
	}
	if _, err := u.UpdateState(ctx, func(s *cyclestate.State) {
		s.FailedAt = MergeFailedRecords(s.FailedAt, state.FailedAt)
		s.CarryoverTodos = MergeTodos(s.CarryoverTodos, state.CarryoverTodos)
	}); err != nil {
		l.warn("Lifecycle.Persist", cycle, CodePersistFailed, "state update failed: "+err.Error(), map[string]string{"step": "update"})
	}
}
