package core

import (
	"context"
	"testing"
)

func TestRunCycle_FleetMode_SkipsGlobalLock(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))
	if _, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(),
		Env:         map[string]string{"EVOLVE_FLEET": "1"},
	}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if st.lockCount != 0 {
		t.Errorf("fleet mode acquired the global project lock %d times, want 0 (R1: fleet cycles must not refuse each other on the coarse lock)", st.lockCount)
	}
}

func TestRunCycle_Default_AcquiresGlobalLock(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if st.lockCount != 1 {
		t.Errorf("default mode acquired the global lock %d times, want 1", st.lockCount)
	}
}
