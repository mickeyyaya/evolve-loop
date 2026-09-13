package carryover

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// Store is the legacy whole-state write (the storage port, at the point of use).
type Store interface {
	WriteState(ctx context.Context, s cyclestate.State) error
}

// Updater is the serialized read-modify-write a store may offer; when it
// does, only the two failure-learning arrays are merged into the disk copy.
type Updater interface {
	UpdateState(ctx context.Context, mutate func(*cyclestate.State)) (cyclestate.State, error)
}

// Persist lands this run's failure-learning arrays: through the store's
// Updater when it has one (FailedAt and CarryoverTodos merged into the disk
// state, LastCycleNumber untouched — preserved), else the whole state through
// WriteState. A failure is one CARRYOVER_PERSIST_FAILED WARN naming the step,
// stamped with the state's LastCycleNumber as the event cycle; the in-memory
// state stands. A nil state is inert; a nil store is a programming error and
// panics (the orchestrator's store is never nil by construction).
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
