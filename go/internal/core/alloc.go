package core

import (
	"context"
	"fmt"
	"math"
)

// StateUpdater is the optional serialized read-modify-write capability the cycle-number allocator needs.
type StateUpdater interface {
	UpdateState(ctx context.Context, mutate func(*State)) (State, error)
}

// AllocateCycleNumber mints the next cycle number through the serialized RMW, so no two allocators receive the same number.
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

// allocateCycle folds the highest occupied Go ACS cycle package into the lease. It syncs the in-memory
// lease so the cycle-end persist cannot clobber it.
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

// persistCycleEndState never rolls back the monotonic lease and revision, because the in-memory copy
// is stale once a concurrent run allocates.
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
		s.StateRevision = diskRev
		// Under EVOLVE_FLEET the global lock is skipped, so union a peer run's concurrent outcome records.
		s.FailedAt = mergeFailedRecords(diskFailed, state.FailedAt)
		s.CarryoverTodos = mergeCarryoverTodos(diskCarry, state.CarryoverTodos)
	})
	return err
}
