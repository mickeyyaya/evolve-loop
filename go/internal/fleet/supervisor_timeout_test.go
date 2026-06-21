package fleet

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestSupervisor_PerCycleTimeoutPropagatesDeadline: a positive CycleTimeout gives
// each launch a deadline-bearing context.
func TestSupervisor_PerCycleTimeoutPropagatesDeadline(t *testing.T) {
	var hadDeadline bool
	s := &Supervisor{
		CycleTimeout: time.Minute,
		Launch: func(ctx context.Context, _ CycleSpec) (int, error) {
			_, hadDeadline = ctx.Deadline()
			return 0, nil
		},
	}
	s.Run(context.Background(), []CycleSpec{{GoalHash: "a"}})
	if !hadDeadline {
		t.Error("launch ctx had no deadline; CycleTimeout should impose one")
	}
}

// TestSupervisor_ZeroTimeoutImposesNoDeadline: CycleTimeout=0 leaves the parent
// context untouched (opt-in deadline).
func TestSupervisor_ZeroTimeoutImposesNoDeadline(t *testing.T) {
	hadDeadline := true
	s := &Supervisor{
		CycleTimeout: 0,
		Launch: func(ctx context.Context, _ CycleSpec) (int, error) {
			_, hadDeadline = ctx.Deadline()
			return 0, nil
		},
	}
	s.Run(context.Background(), []CycleSpec{{GoalHash: "a"}})
	if hadDeadline {
		t.Error("CycleTimeout=0 should leave the parent ctx deadline-free")
	}
}

// TestSupervisor_PerCycleTimeoutReapsHungLaunch: a wedged child that only unblocks
// on cancellation is reaped at the deadline instead of hanging the wave forever
// (the BLOCKER from the large-scale readiness audit).
func TestSupervisor_PerCycleTimeoutReapsHungLaunch(t *testing.T) {
	s := &Supervisor{
		CycleTimeout: 30 * time.Millisecond,
		Launch: func(ctx context.Context, _ CycleSpec) (int, error) {
			<-ctx.Done()
			return -1, ctx.Err()
		},
	}
	done := make(chan []Result, 1)
	go func() { done <- s.Run(context.Background(), []CycleSpec{{GoalHash: "hung"}}) }()
	select {
	case res := <-done:
		if len(res) != 1 || !errors.Is(res[0].Err, context.DeadlineExceeded) {
			t.Fatalf("result = %+v, want one result with DeadlineExceeded", res)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Supervisor.Run hung well past the per-cycle timeout")
	}
}
