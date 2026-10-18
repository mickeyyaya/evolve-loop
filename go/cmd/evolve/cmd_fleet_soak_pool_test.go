package main

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
)

func barrierLaunch(width int, rc int) (fleet.LaunchFn, func() []fleet.CycleSpec) {
	var mu sync.Mutex
	var specs []fleet.CycleSpec
	gate := make(chan struct{})
	return func(_ context.Context, spec fleet.CycleSpec) (int, error) {
			mu.Lock()
			specs = append(specs, spec)
			wait := gate
			if len(specs)%width == 0 {
				close(gate)
				gate = make(chan struct{})
			}
			mu.Unlock()
			<-wait
			return rc, nil
		}, func() []fleet.CycleSpec {
			mu.Lock()
			defer mu.Unlock()
			return append([]fleet.CycleSpec(nil), specs...)
		}
}

func steppedClock() func() time.Time {
	var mu sync.Mutex
	at := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		at = at.Add(time.Second)
		return at
	}
}

func TestSoakPool_TheEnforcedPoolFillsEverySlotAndRunsFourLanesPerSlot(t *testing.T) {
	t.Parallel()
	launch, specs := barrierLaunch(4, 0)
	rep := soakPool(context.Background(), 4, "g", launch, steppedClock())

	if rep.Lanes != 16 || rep.OK != 16 {
		t.Fatalf("report = %+v, want 16 lanes, all ok (run_max_lanes is 4 x count)", rep)
	}
	if rep.Peak != 4 {
		t.Errorf("peak in-flight = %d, want 4 (every slot busy)", rep.Peak)
	}
	if rep.Wall <= 0 || rep.Busy <= 0 || rep.Utilization() <= 0 || rep.Utilization() > 1 {
		t.Errorf("wall = %v busy = %v utilization = %v, want a positive share of at most 1", rep.Wall, rep.Busy, rep.Utilization())
	}
	if got := rep.Throughput(); got != float64(rep.Lanes)/rep.Wall.Minutes() {
		t.Errorf("throughput = %v, want lanes per minute", got)
	}
	for _, s := range specs() {
		if s.GoalHash != "g" || s.Env[ipcenv.FleetScopeKey] != "" || s.Env[ipcenv.FleetKey] != "1" {
			t.Errorf("soak spec = %+v, want the soak goal, fleet mode and no inbox scope", s)
		}
	}
	if !rep.Passed(4) {
		t.Errorf("Passed(4) = false for %+v", rep)
	}
}

func TestSoakPool_AFailedLaneOrAnUnfilledSlotFailsTheRow(t *testing.T) {
	t.Parallel()
	launch, _ := barrierLaunch(2, 1)
	rep := soakPool(context.Background(), 2, "g", launch, steppedClock())
	if rep.OK != 0 || rep.Passed(2) {
		t.Errorf("report = %+v passed = %v, want no ok lane and a failed row", rep, rep.Passed(2))
	}
	if (soakPoolReport{Lanes: 4, OK: 4, Peak: 1}).Passed(2) {
		t.Error("a pool that never filled its two slots passed")
	}
	if got := (soakPoolReport{}).Utilization(); got != 0 {
		t.Errorf("utilization of an empty run = %v, want 0", got)
	}
	if _, ok := soakHeartbeat(orchestrator.Lane{Key: "soak-1"}); ok {
		t.Error("a simulated soak lane has a heartbeat, want none: the soak has no cycle signals to read")
	}
	if got := (soakPoolReport{}).Throughput(); got != 0 {
		t.Errorf("throughput of an empty run = %v, want 0", got)
	}
}

func TestFleetSoak_PoolFlagAddsTheEnforcedPoolRow(t *testing.T) {
	launch, _ := barrierLaunch(2, 0)
	soakLaunchFn = launch
	soakKiller = func(context.Context, string) error { return nil }
	t.Cleanup(func() { soakLaunchFn, soakKiller = nil, nil })

	var out, errBuf bytes.Buffer
	rc := runFleetSoak([]string{"--count", "2", "--pool", "--evolve-dir", t.TempDir()}, nil, &out, &errBuf)
	if rc != 0 {
		t.Fatalf("rc = %d\nstdout:\n%s\nstderr:\n%s", rc, out.String(), errBuf.String())
	}
	row := ""
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "| 5 — pool at enforce") {
			row = line
		}
	}
	want := "8 lanes, peak in-flight 2 of 2"
	if !strings.Contains(row, want) || !strings.Contains(row, "lanes/min") || !strings.Contains(row, "utilization") || !strings.HasSuffix(row, "| PASS |") {
		t.Errorf("row 5 = %q, want %q with throughput and utilization, PASS", row, want)
	}
}
