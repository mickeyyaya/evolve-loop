package core

import (
	"context"
	"fmt"
	"math"
)

// alloc.go — CA.4 (concurrency-factory plan, Track C-A): the cycle-number
// allocation lease. Today's `LastCycleNumber+1` is safe only because the
// project lock forbids concurrent runs; the fleet supervisor (CE) lifts
// that, so numbers must come from an atomic allocation through the CA.3
// serialized UpdateState RMW. Semantics:
//
//   - allocate = max(lastCompleted, lastAllocated, highest occupied Go ACS
//     cycle package) + 1, persisted before any run artifact exists under that
//     number;
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

// AllocateCycleNumber mints the next state-derived cycle number through the
// serialized RMW: no two allocators — goroutines or processes — can receive
// the same number. RunCycle additionally supplies the repository's occupied
// ACS package floor through allocateCycleNumberAbove.
func AllocateCycleNumber(ctx context.Context, su StateUpdater) (int, error) {
	return allocateCycleNumberAbove(ctx, su, 0)
}

func allocateCycleNumberAbove(ctx context.Context, su StateUpdater, floor int) (int, error) {
	var allocationErr error
	st, err := su.UpdateState(ctx, func(s *State) {
		base := max(s.LastCycleNumber, s.LastAllocatedCycleNumber, floor)
		next, err := nextCycleNumber(base)
		if err != nil {
			allocationErr = err
			return
		}
		s.LastAllocatedCycleNumber = next
	})
	if err != nil {
		return 0, fmt.Errorf("allocate cycle number: %w", err)
	}
	if allocationErr != nil {
		return 0, allocationErr
	}
	return st.LastAllocatedCycleNumber, nil
}

func nextCycleNumber(base int) (int, error) {
	if base == math.MaxInt {
		return 0, fmt.Errorf("allocate cycle number: cycle number exhausted at %d", base)
	}
	return base + 1, nil
}

// allocateCycle is RunCycle's allocation step. It discovers the highest
// occupied canonical Go ACS package before minting, then folds that floor into
// the serialized lease when storage supports it. Legacy storage still advances
// above both LastCycleNumber and the source floor. On the lease path the
// in-memory state is synced with the persisted lease fields so the cycle-end
// persist does not clobber them with stale zeros.
func (o *Orchestrator) allocateCycle(ctx context.Context, state *State, projectRoot string) (int, error) {
	floor, err := sourceCycleFloor(projectRoot)
	if err != nil {
		return 0, fmt.Errorf("discover source cycle floor: %w", err)
	}
	su, ok := o.storage.(StateUpdater)
	if !ok {
		return nextCycleNumber(max(state.LastCycleNumber, floor))
	}
	n, err := allocateCycleNumberAbove(ctx, su, floor)
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
		diskLease, diskRev, diskCycle := s.LastAllocatedCycleNumber, s.StateRevision, s.LastCycleNumber
		diskFailed, diskCarry := s.FailedAt, s.CarryoverTodos
		*s = state
		s.LastAllocatedCycleNumber = max(s.LastAllocatedCycleNumber, diskLease)
		s.LastCycleNumber = max(s.LastCycleNumber, diskCycle)
		s.StateRevision = diskRev // UpdateState's own ++ owns the bump
		// Under EVOLVE_FLEET the global lock is skipped, so a peer run may have
		// appended outcome records after this run loaded; union them so the blind
		// *s=state cannot drop a concurrent peer's FailedAt/CarryoverTodos.
		s.FailedAt = mergeFailedRecords(diskFailed, state.FailedAt)
		s.CarryoverTodos = mergeCarryoverTodos(diskCarry, state.CarryoverTodos)
	})
	return err
}
