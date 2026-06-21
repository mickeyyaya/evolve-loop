package core

import (
	"context"
	"fmt"
)

// alloc.go — CA.4 (concurrency-factory plan, Track C-A): the cycle-number
// allocation lease. Today's `LastCycleNumber+1` is safe only because the
// project lock forbids concurrent runs; the fleet supervisor (CE) lifts
// that, so numbers must come from an atomic allocation through the CA.3
// serialized UpdateState RMW. Semantics:
//
//   - allocate = max(lastCompleted, lastAllocated) + 1, persisted before
//     any run artifact exists under that number;
//   - a crashed run BURNS its number (the gap is intentional — a burned
//     number may have orphaned artifacts; reuse would mix two runs' state);
//   - resume never re-allocates: RunCycleFromPhase carries the cycle from
//     the run record (ResumePoint).

// StateUpdater is the optional storage capability the allocator needs —
// satisfied by adapters/storage.FilesystemStorage (CA.3). Kept separate
// from the Storage port so every existing fake/adapter stays valid; the
// orchestrator falls back to the legacy path when absent (nil-seam
// convention: absence ⇒ byte-identical behavior).
type StateUpdater interface {
	UpdateState(ctx context.Context, mutate func(*State)) (State, error)
}

// AllocateCycleNumber mints the next cycle number through the serialized
// RMW: no two allocators — goroutines or processes — can receive the same
// number.
func AllocateCycleNumber(ctx context.Context, su StateUpdater) (int, error) {
	st, err := su.UpdateState(ctx, func(s *State) {
		s.LastAllocatedCycleNumber = max(s.LastCycleNumber, s.LastAllocatedCycleNumber) + 1
	})
	if err != nil {
		return 0, fmt.Errorf("allocate cycle number: %w", err)
	}
	return st.LastAllocatedCycleNumber, nil
}

// allocateCycle is RunCycle's allocation step: the lease when the storage
// supports it, the legacy LastCycleNumber+1 otherwise. On the lease path
// the in-memory state is synced with the persisted lease fields so the
// cycle-end persist does not clobber them with stale zeros.
func (o *Orchestrator) allocateCycle(ctx context.Context, state *State) (int, error) {
	su, ok := o.storage.(StateUpdater)
	if !ok {
		return state.LastCycleNumber + 1, nil
	}
	n, err := AllocateCycleNumber(ctx, su)
	if err != nil {
		return 0, err
	}
	state.LastAllocatedCycleNumber = n
	return n, nil
}

// persistCycleEndState writes the run's final state. On StateUpdater
// storages it goes through the serialized RMW with a lease max-merge: the
// run's in-memory copy of LastAllocatedCycleNumber is stale the moment a
// CONCURRENT run allocates, so a blind WriteState would roll the lease
// back and the next allocator would re-mint a number another run holds
// (review-named CA.4 clobber). The monotonic fields — lease and
// stateRevision — are never rolled back; every other field is the run's
// own outcome and lands as-is. Cross-run merge of outcome fields
// (FailedAt etc.) is CB/CC scope — today the project lock still prevents
// two concurrent RunCycles.
func (o *Orchestrator) persistCycleEndState(ctx context.Context, state State) error {
	su, ok := o.storage.(StateUpdater)
	if !ok {
		return o.storage.WriteState(ctx, state)
	}
	_, err := su.UpdateState(ctx, func(s *State) {
		diskLease, diskRev := s.LastAllocatedCycleNumber, s.StateRevision
		diskFailed, diskCarry := s.FailedAt, s.CarryoverTodos
		*s = state
		s.LastAllocatedCycleNumber = max(s.LastAllocatedCycleNumber, diskLease)
		s.StateRevision = diskRev // UpdateState's own ++ owns the bump
		// Under EVOLVE_FLEET the global lock is skipped, so a peer run may have
		// appended outcome records after this run loaded; union them so the blind
		// *s=state cannot drop a concurrent peer's FailedAt/CarryoverTodos.
		s.FailedAt = mergeFailedRecords(diskFailed, state.FailedAt)
		s.CarryoverTodos = mergeCarryoverTodos(diskCarry, state.CarryoverTodos)
	})
	return err
}
