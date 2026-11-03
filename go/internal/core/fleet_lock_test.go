package core

import (
	"context"
	"testing"
)

func TestRunCycle_FleetMode_SkipsGlobalLock(t *testing.T) {
	t.Parallel()

	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil), WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}))
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
	t.Parallel()

	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if st.lockCount != 1 {
		t.Errorf("default mode acquired the global lock %d times, want 1", st.lockCount)
	}
}

func TestRunCycleFromPhase_AFleetLaneResumesWhileAnotherLaneHoldsTheProjectLock(t *testing.T) {
	t.Parallel()
	projectRoot, st := resumedCycleNine(t)
	st.lockHeld = true
	runners := buildRunners(nil)
	o := NewOrchestrator(st, &fakeLedger{}, runners, WithWorktreeProvisioner(&fakeWorktree{path: st.cycleState.ActiveWorktree}))
	req := CycleRequest{ProjectRoot: projectRoot, GoalHash: "g", Env: map[string]string{"EVOLVE_FLEET": "1", "EVOLVE_FLEET_SCOPE": "todo-a"}}

	if _, err := o.RunCycleFromPhase(context.Background(), req, &ResumePoint{Phase: string(PhaseBuild), CycleID: 9}); err != nil {
		t.Fatalf("RunCycleFromPhase in a fleet lane: %v (a resumed lane must not refuse on the project lock of a sibling lane)", err)
	}
	if st.lockCount != 0 || runners[PhaseBuild].(*fakeRunner).calls == 0 {
		t.Errorf("lock taken %d times and build ran %d times; want 0 locks and the resumed build to run", st.lockCount, runners[PhaseBuild].(*fakeRunner).calls)
	}
}

func TestRunCycleFromPhase_OutsideAFleetTheResumeTakesTheProjectLock(t *testing.T) {
	t.Parallel()
	projectRoot, st := resumedCycleNine(t)
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil), WithWorktreeProvisioner(&fakeWorktree{path: st.cycleState.ActiveWorktree}))

	if _, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: projectRoot, GoalHash: "g"}, &ResumePoint{Phase: string(PhaseBuild), CycleID: 9}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	if st.lockCount != 1 || st.lockHeld {
		t.Errorf("lockCount = %d, held after return = %v; want one lock, released", st.lockCount, st.lockHeld)
	}
}

func TestRunCycleFromPhase_ASequentialResumeThatInheritedTheFleetFlagWithoutAScopeTakesTheProjectLock(t *testing.T) {
	t.Parallel()
	for name, env := range map[string]map[string]string{
		"no scope":    {"EVOLVE_FLEET": "1"},
		"blank scope": {"EVOLVE_FLEET": "1", "EVOLVE_FLEET_SCOPE": " , "},
		"scope only":  {"EVOLVE_FLEET_SCOPE": "todo-a"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			projectRoot, st := resumedCycleNine(t)
			o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil), WithWorktreeProvisioner(&fakeWorktree{path: st.cycleState.ActiveWorktree}))

			if _, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: projectRoot, GoalHash: "g", Env: env}, &ResumePoint{Phase: string(PhaseBuild), CycleID: 9}); err != nil {
				t.Fatalf("RunCycleFromPhase: %v", err)
			}
			if st.lockCount != 1 {
				t.Errorf("env %v took the project lock %d times, want 1: only a lane with a fleet scope may skip it", env, st.lockCount)
			}
		})
	}
}
