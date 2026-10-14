package core

import (
	"context"
	"fmt"
	"math"
)

type StateUpdater interface {
	UpdateState(ctx context.Context, mutate func(*State)) (State, error)
}

func AllocateCycleNumber(ctx context.Context, su StateUpdater) (int, error) {
	return allocateCycleNumberAbove(ctx, su, 0)
}

func allocateCycleNumberAbove(ctx context.Context, su StateUpdater, floor int) (int, error) {
	var allocationErr error
	st, err := su.UpdateState(ctx, func(s *State) {
		base := max(s.HighestCycleNumber(), floor)
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
		s.FailedAt = mergeFailedRecords(diskFailed, state.FailedAt)
		s.CarryoverTodos = mergeCarryoverTodos(diskCarry, state.CarryoverTodos)
	})
	return err
}
