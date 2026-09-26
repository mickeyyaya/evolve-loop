package fleet

import (
	"context"
	"io"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

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
