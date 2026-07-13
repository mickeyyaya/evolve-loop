package fleet

import (
	"context"
	"io"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
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
		sawFleet = spec.Env[ipcenv.FleetKey]
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

// TestFreshnessGateFnTypes_Named names the dispatch freshness gate's bare
// contract types (FreshnessProbeFn, RefillFn, FreshnessSkip — assigned inline
// in freshness_test.go but never named there) and pins their contract: a probe
// verdict of stale flows into a FreshnessSkip carrying the probe's reason, and
// an ok=false RefillFn leaves the freed slot empty.
func TestFreshnessGateFnTypes_Named(t *testing.T) {
	var probe FreshnessProbeFn = func(string) TaskFreshness {
		return TaskFreshness{Fresh: false, Reason: "consumed: processed"}
	}
	var refill RefillFn = func(map[string]bool) (CycleSpec, bool) {
		return CycleSpec{}, false
	}
	kept, skipped := FreshenSpecs([]CycleSpec{{Scope: []string{"task-x"}}}, probe, refill, io.Discard)
	if len(kept) != 0 {
		t.Fatalf("stale-only wave with empty backlog must keep nothing, got %+v", kept)
	}
	var sk FreshnessSkip
	if len(skipped) != 1 {
		t.Fatalf("want exactly one skip record, got %+v", skipped)
	}
	sk = skipped[0]
	if sk.TaskID != "task-x" || sk.Reason != "consumed: processed" {
		t.Fatalf("FreshnessSkip must carry the probe's id + reason, got %+v", sk)
	}
}
