package core

import (
	"context"
	"testing"
)

// TestRunCycle_FleetMode_SkipsGlobalLock pins ADR-0049 S6 / root-cause R1: under
// the fleet supervisor (EVOLVE_FLEET=1) a cycle must NOT take the whole-cycle
// global project lock (LOCK_NB), which refuses concurrent runs. Concurrent fleet
// cycles run in separate processes, each isolated by its per-run worktree +
// workspace and serialized on every SHARED resource by that resource's own flock
// (state.json via UpdateState/withStateLock, the ledger chain, the .evolve/
// ship.lock integrator) — the safety nets S2–S5 put in place. RED before the
// fleet gate (lockCount=1), GREEN after (0).
func TestRunCycle_FleetMode_SkipsGlobalLock(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))
	if _, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: "/tmp/p",
		Env:         map[string]string{"EVOLVE_FLEET": "1"},
	}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if st.lockCount != 0 {
		t.Errorf("fleet mode acquired the global project lock %d times, want 0 (R1: fleet cycles must not refuse each other on the coarse lock)", st.lockCount)
	}
}

// TestRunCycle_Default_AcquiresGlobalLock: the live sequential loop (no
// EVOLVE_FLEET) keeps the whole-cycle global lock — byte-identical to pre-S6.
func TestRunCycle_Default_AcquiresGlobalLock(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if st.lockCount != 1 {
		t.Errorf("default mode acquired the global lock %d times, want 1", st.lockCount)
	}
}
