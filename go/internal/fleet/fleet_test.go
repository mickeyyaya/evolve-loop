package fleet

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// TestSupervisor_Validate: a misconfigured supervisor (nil LaunchFn) is caught
// by Validate BEFORE any work is scheduled — one loud error, not N late ones.
func TestSupervisor_Validate(t *testing.T) {
	if err := (&Supervisor{}).Validate(); !errors.Is(err, errNoLaunch) {
		t.Errorf("Validate() nil Launch = %v, want errNoLaunch", err)
	}
	ok := &Supervisor{Launch: func(context.Context, CycleSpec) (int, error) { return 0, nil }}
	if err := ok.Validate(); err != nil {
		t.Errorf("Validate() configured = %v, want nil", err)
	}
}

// TestSupervisor_LaunchesAllWithFleetEnv: every spec is launched, in input order,
// each forced to EVOLVE_FLEET=1 (so the orchestrator skips the global lock), and
// its exit code is collected.
func TestSupervisor_LaunchesAllWithFleetEnv(t *testing.T) {
	var mu sync.Mutex
	gotFleet := map[string]string{}
	s := &Supervisor{Launch: func(_ context.Context, spec CycleSpec) (int, error) {
		mu.Lock()
		gotFleet[spec.GoalHash] = spec.Env[ipcenv.FleetKey]
		mu.Unlock()
		switch spec.GoalHash {
		case "b":
			return 2, nil
		default:
			return 0, nil
		}
	}}
	specs := []CycleSpec{{GoalHash: "a"}, {GoalHash: "b"}, {GoalHash: "c"}}
	res := s.Run(context.Background(), specs)

	if len(res) != 3 {
		t.Fatalf("got %d results, want 3", len(res))
	}
	for i, want := range []int{0, 2, 0} {
		if res[i].Index != i || res[i].ExitCode != want || res[i].Err != nil {
			t.Errorf("result[%d]=%+v, want index=%d exit=%d err=nil", i, res[i], i, want)
		}
	}
	for _, h := range []string{"a", "b", "c"} {
		if gotFleet[h] != "1" {
			t.Errorf("cycle %q launched with EVOLVE_FLEET=%q, want 1 (must arm fleet mode)", h, gotFleet[h])
		}
	}
}

// TestSupervisor_BoundedConcurrency: at most Concurrency cycles run at once.
func TestSupervisor_BoundedConcurrency(t *testing.T) {
	var inFlight, maxSeen int32
	release := make(chan struct{})
	var started sync.WaitGroup
	started.Add(5)
	s := &Supervisor{
		Concurrency: 2,
		Launch: func(_ context.Context, _ CycleSpec) (int, error) {
			n := atomic.AddInt32(&inFlight, 1)
			for {
				m := atomic.LoadInt32(&maxSeen)
				if n <= m || atomic.CompareAndSwapInt32(&maxSeen, m, n) {
					break
				}
			}
			started.Done()
			<-release
			atomic.AddInt32(&inFlight, -1)
			return 0, nil
		},
	}
	specs := make([]CycleSpec, 5)
	done := make(chan []Result, 1)
	go func() { done <- s.Run(context.Background(), specs) }()
	// With limit 2, only 2 can be in-flight; the other 3 block on the semaphore.
	// Give the 2 a moment to peak, then release.
	for atomic.LoadInt32(&inFlight) < 2 {
	}
	close(release)
	<-done
	if got := atomic.LoadInt32(&maxSeen); got > 2 {
		t.Errorf("max concurrent launches = %d, want <= 2", got)
	}
}

// TestSupervisor_NilLaunch_ErrorsPerSpec: a misconfigured supervisor surfaces an
// error per spec rather than silently doing nothing.
func TestSupervisor_NilLaunch_ErrorsPerSpec(t *testing.T) {
	s := &Supervisor{}
	res := s.Run(context.Background(), []CycleSpec{{GoalHash: "x"}, {GoalHash: "y"}})
	if len(res) != 2 {
		t.Fatalf("got %d results, want 2", len(res))
	}
	for i := range res {
		if res[i].Err == nil {
			t.Errorf("result[%d] has nil err; a nil LaunchFn must fail loud", i)
		}
	}
}

// TestSupervisor_DoesNotMutateCallerEnv: the EVOLVE_FLEET forcing must not leak
// into the caller's spec.Env map.
func TestSupervisor_DoesNotMutateCallerEnv(t *testing.T) {
	callerEnv := map[string]string{"EVOLVE_CLI": "codex"}
	s := &Supervisor{Launch: func(_ context.Context, _ CycleSpec) (int, error) { return 0, nil }}
	s.Run(context.Background(), []CycleSpec{{GoalHash: "a", Env: callerEnv}})
	if _, leaked := callerEnv[ipcenv.FleetKey]; leaked {
		t.Errorf("EVOLVE_FLEET leaked into the caller's env map: %v", callerEnv)
	}
}

// TestSupervisor_EmptySpecs: no specs → no launches, empty results.
func TestSupervisor_EmptySpecs(t *testing.T) {
	var launched int32
	s := &Supervisor{Launch: func(_ context.Context, _ CycleSpec) (int, error) {
		atomic.AddInt32(&launched, 1)
		return 0, nil
	}}
	if res := s.Run(context.Background(), nil); len(res) != 0 {
		t.Errorf("empty specs got %d results, want 0", len(res))
	}
	if launched != 0 {
		t.Errorf("launched %d cycles for empty specs, want 0", launched)
	}
}
