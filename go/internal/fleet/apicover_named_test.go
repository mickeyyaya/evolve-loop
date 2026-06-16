package fleet

import (
	"context"
	"sync"
	"testing"
)

// TestLaunchFn_InvokedWithFleetEnvForced names the fleet.LaunchFn func type
// (assigned to Supervisor.Launch but the bare type is never named in a test) and
// pins the contract between Supervisor.Run and the function it drives: the
// supervisor calls the LaunchFn once per spec with EVOLVE_FLEET=1 forced into
// spec.Env (so the launched cycle skips the global project lock), and propagates
// the returned exit code.
func TestLaunchFn_InvokedWithFleetEnvForced(t *testing.T) {
	var mu sync.Mutex
	var calls int
	var sawFleet string
	var fn LaunchFn = func(_ context.Context, spec CycleSpec) (int, error) {
		mu.Lock()
		calls++
		sawFleet = spec.Env[fleetEnvKey]
		mu.Unlock()
		return 7, nil
	}
	s := &Supervisor{Launch: fn}
	res := s.Run(context.Background(), []CycleSpec{{GoalHash: "a"}})
	if calls != 1 {
		t.Fatalf("LaunchFn invoked %d times, want 1", calls)
	}
	if sawFleet != "1" {
		t.Fatalf("LaunchFn saw EVOLVE_FLEET=%q, want 1 (Run must force fleet mode)", sawFleet)
	}
	if res[0].ExitCode != 7 {
		t.Fatalf("ExitCode = %d, want 7 (LaunchFn return must propagate)", res[0].ExitCode)
	}
}
