package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/hostpressure"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

var hostBase = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

const lockWaitBoundS = 600

type lineSink struct {
	mu    sync.Mutex
	lines []hostSampleLine
}

func (s *lineSink) record(raw []byte) error {
	var line hostSampleLine
	if err := json.Unmarshal(raw, &line); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lines = append(s.lines, line)
	return nil
}

func (s *lineSink) all() []hostSampleLine {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]hostSampleLine(nil), s.lines...)
}

func waitsOf(waits ...time.Duration) func() time.Duration {
	i := 0
	return func() time.Duration {
		w := waits[min(i, len(waits)-1)]
		i++
		return w
	}
}

func boundedSampler(rig *sampleRig, stage string, waits ...time.Duration) *hostSampler {
	s := rig.sampler(stage, 1)
	s.cfg.LockWaitMax = lockWaitBoundS * time.Second
	s.lockWait = waitsOf(waits...)
	return s
}

func TestHostPressureLockWait_ShadowLineNamesAWidenThatTheLockWaitHeld(t *testing.T) {
	t.Parallel()
	rig := &sampleRig{readings: []hostpressure.Reading{heavyHost, calmHost, calmHost}}
	sink := &lineSink{}
	s := boundedSampler(rig, "shadow", 0, 1200*time.Second, 0)
	s.record = sink.record
	for range 3 {
		s.sample()
	}
	lines := sink.all()
	if len(lines) != 3 {
		t.Fatalf("sample lines = %d, want 3", len(lines))
	}
	if held := lines[1]; !held.LockWaitHeld || held.LockWaitMS != 1200000 || held.Decided != held.Cap || held.Stage != "shadow" {
		t.Errorf("line 2 = %+v, want lock_wait_held true, lock_wait_ms 1200000 and a decided cap equal to the cap: the lock wait of the last window is over the bound of %ds, so the window would not widen", held, lockWaitBoundS)
	}
	if first := lines[0]; first.LockWaitHeld || first.LockWaitMS != 0 || first.Decided >= first.Cap {
		t.Errorf("line 1 = %+v, want a decrease with lock_wait_held false and lock_wait_ms 0", first)
	}
	if cleared := lines[2]; cleared.LockWaitHeld || cleared.Decided != cleared.Cap+1 {
		t.Errorf("line 3 = %+v, want the widening of the old rule once the wait cleared", cleared)
	}
	if len(rig.applied) != 0 {
		t.Errorf("applied = %v, want no cap applied at shadow", rig.applied)
	}
}

func TestHostPressureLockWait_EnforceHoldsTheCapWhileTheWaitIsOverTheBound(t *testing.T) {
	t.Parallel()
	rig := &sampleRig{readings: []hostpressure.Reading{heavyHost, calmHost, calmHost, calmHost}}
	s := boundedSampler(rig, "enforce", 0, 700*time.Second, 601*time.Second, 0)
	for range 4 {
		s.sample()
	}
	if want := []int{6, 7}; len(rig.applied) != len(want) || rig.applied[0] != want[0] || rig.applied[1] != want[1] {
		t.Errorf("applied = %v, want %v: the decrease, no widening while the wait is over the bound, then one widening", rig.applied, want)
	}
}

func TestHostPressureLockWait_HostConfigCarriesThePolicyBound(t *testing.T) {
	t.Parallel()
	a := adaptiveOn("shadow")
	a.LockWaitMaxS = 420
	if got := hostConfig(a, 4, 12).LockWaitMax; got != 420*time.Second {
		t.Errorf("LockWaitMax = %s, want 420s from the policy bound", got)
	}
	a.LockWaitMaxS = 0
	if got := hostConfig(a, 4, 12).LockWaitMax; got != 0 {
		t.Errorf("LockWaitMax = %s, want 0 when the policy bound is 0", got)
	}
}

func tickClockedRig(t *testing.T, readings ...hostpressure.Reading) (*hostRig, *lineSink) {
	t.Helper()
	rig := newHostRig(t, readings...)
	sink := &lineSink{}
	rig.b.hostSamples = sink.record
	var reads atomic.Int64
	read := rig.b.hostReader
	rig.b.hostReader = func() (hostpressure.Reading, error) {
		reads.Add(1)
		return read()
	}
	rig.b.now = func() time.Time { return hostBase.Add(time.Duration(reads.Load()) * 200 * time.Second) }
	return rig, sink
}

func writeRunWait(t *testing.T, evolveDir string, cycle int, at time.Time, waitMS int) {
	t.Helper()
	dir := filepath.Join(evolveDir, "runs", fmt.Sprintf("cycle-%d", cycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := fmt.Sprintf(`{"schema_version":"signal/1.0","seq":%d,"pid":%d,"ts":%q,"cycle":%d,"module":"host","origin":"verifylock.AcquireWithin","kind":"loop.warning","code":"HOST_VERIFY_LOCK_WAIT","severity":"INFO","reason":"waited","fields":{"outcome":"acquired","source":"build-floor","wait_ms":"%d"}}`+"\n",
		cycle, 50000+cycle, at.Format(time.RFC3339Nano), cycle, waitMS)
	if err := os.WriteFile(filepath.Join(dir, "signals.ndjson"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newBoundedWindow() hostSeat {
	w := orchestrator.NewWindow(orchestrator.WindowConfig{Start: 4, Min: 1, Sized: 4}, nil)
	return hostSeat{window: w, intake: orchestrator.NewIntake(4)}
}

func boundedFleet(stage string) policy.FleetConfig {
	fc := adaptiveFleet(stage)
	fc.Orchestrator.Adaptive.LockWaitMaxS = lockWaitBoundS
	return fc
}

func TestStartHostPressure_ReadsTheLockWaitFromTheRunSignalLogsOfTheFleet(t *testing.T) {
	t.Parallel()
	rig, sink := tickClockedRig(t, heavyHost, calmHost)
	nearSecondSample := hostBase.Add(395 * time.Second)
	writeRunWait(t, rig.dir, 7, nearSecondSample, 450000)
	writeRunWait(t, rig.dir, 8, nearSecondSample, 450000)
	launch := func(_ context.Context, _ fleet.CycleSpec) (int, error) {
		rig.tickAndAwaitRead(t)
		rig.tickAndAwaitRead(t)
		return 0, nil
	}
	rig.b.orchestratedRun(boundedFleet("shadow"), launch, 2)(context.Background(), []fleet.Todo{{ID: "first"}}, noResupply)
	lines := sink.all()
	if len(lines) < 2 {
		t.Fatalf("sample lines = %d, want 2", len(lines))
	}
	if held := lines[1]; !held.LockWaitHeld || held.LockWaitMS != 900000 || held.Decided != held.Cap {
		t.Errorf("line 2 = %+v, want lock_wait_ms 900000 (450 s of cycle 7 plus 450 s of cycle 8, each under the bound of %d s and the sum over it) and the widening held", held, lockWaitBoundS)
	}
}

func TestStartHostPressure_AWaitOutsideTheLastWindowOrUnderTheBoundDoesNotHoldTheWidening(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		at     time.Time
		waitMS int
		wantMS int64
	}{
		"a wait two hours before the sample":       {hostBase.Add(-2 * time.Hour), 900000, 0},
		"a wait under the bound in the window":     {hostBase.Add(395 * time.Second), 100000, 100000},
		"a wait at the bound in the window":        {hostBase.Add(395 * time.Second), lockWaitBoundS * 1000, lockWaitBoundS * 1000},
		"a wait that ends after the sample (skew)": {hostBase.Add(48 * time.Hour), 900000, 0},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rig, sink := tickClockedRig(t, heavyHost, calmHost)
			writeRunWait(t, rig.dir, 7, c.at, c.waitMS)
			stop := rig.b.startHostPressure(context.Background(), boundedFleet("shadow"), newBoundedWindow())
			rig.tickAndAwaitRead(t)
			rig.tickAndAwaitRead(t)
			stop()
			lines := sink.all()
			if len(lines) != 2 {
				t.Fatalf("sample lines = %d, want 2", len(lines))
			}
			if got := lines[1]; got.LockWaitHeld || got.LockWaitMS != c.wantMS || got.Decided != got.Cap+1 {
				t.Errorf("line 2 = %+v, want the old widening %d -> %d, lock_wait_held false and lock_wait_ms %d", got, got.Cap, got.Cap+1, c.wantMS)
			}
		})
	}
}
