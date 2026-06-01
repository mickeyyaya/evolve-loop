package core

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

// TestOrchestrator_WithCatalogRefresher_CalledOnceBestEffort proves the
// cycle-start hook: the injected refresher runs exactly once, and an error it
// returns is best-effort (WARN) — it must NOT fail the cycle.
func TestOrchestrator_WithCatalogRefresher_CalledOnceBestEffort(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	var calls int32
	o := NewOrchestrator(st, led, buildRunners(nil), WithCatalogRefresher(func(context.Context) error {
		atomic.AddInt32(&calls, 1)
		return errors.New("refresh boom") // best-effort: must not abort the cycle
	}))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p", GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle must not fail on refresher error: %v", err)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Errorf("verdict=%s, want PASS", res.FinalVerdict)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("refresher called %d times, want exactly 1", n)
	}
}
