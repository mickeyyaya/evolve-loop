package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/hostpressure"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var (
	heavyHost = hostpressure.Reading{Load1: 28, CPUs: 14, MemAvailablePct: 80}
	calmHost  = hostpressure.Reading{Load1: 7, CPUs: 14, MemAvailablePct: 80}
	errHost   = errors.New("host read failed")
)

func adaptiveOn(stage string) policy.AdaptiveConfig {
	return policy.AdaptiveConfig{Enabled: true, Stage: stage, LoadHigh: 1.5, LoadLow: 1.0, MemMinPct: 15, SwapMaxMB: 4096,
		DecreaseFactor: 0.5, Step: 1, CooldownS: 60, IncreaseCooldownS: 180, SampleS: 15, SmoothSamples: 1}
}

type sampleRig struct {
	readings []hostpressure.Reading
	errs     []error
	applied  []int
	events   []signalcenter.Event
	calls    int
	clock    time.Time
	lines    [][]byte
	warns    []error
	failLine error
}

func (r *sampleRig) sampler(stage string, smooth int) *hostSampler {
	a := adaptiveOn(stage)
	a.SmoothSamples = smooth
	cfg := hostConfig(a, 4, 12)
	return &hostSampler{
		cfg: cfg, stage: stage, state: hostpressure.NewState(cfg.Ceiling), goos: "testos",
		read: func() (hostpressure.Reading, error) {
			i := r.calls
			r.calls++
			if i < len(r.errs) && r.errs[i] != nil {
				return hostpressure.Reading{}, r.errs[i]
			}
			return r.readings[i], nil
		},
		record: func(b []byte) error {
			r.lines = append(r.lines, b)
			return r.failLine
		},
		warn:    func(err error) { r.warns = append(r.warns, err) },
		apply:   func(n int) { r.applied = append(r.applied, n) },
		publish: func(e signalcenter.Event) { r.events = append(r.events, e) },
		now:     func() time.Time { r.clock = r.clock.Add(200 * time.Second); return r.clock },
	}
}

func TestHostSampler_EnforceAppliesADecreaseAndSignalsItApplied(t *testing.T) {
	t.Parallel()
	rig := &sampleRig{readings: []hostpressure.Reading{heavyHost}}
	rig.sampler("enforce", 1).sample()
	if len(rig.applied) != 1 || rig.applied[0] != 6 {
		t.Fatalf("applied = %v, want [6]", rig.applied)
	}
	if len(rig.events) != 1 || rig.events[0].Code != orchestrator.CodeHostPressure {
		t.Fatalf("events = %+v, want one host pressure event", rig.events)
	}
	f := rig.events[0].Fields
	if f["old"] != "12" || f["new"] != "6" || f["sized"] != "12" || f["applied"] != "true" || f["stage"] != "enforce" || f["cause"] != "pressure_high" || f["load_per_core"] != "2" {
		t.Errorf("fields = %v", f)
	}
}

func TestHostSampler_EnforceHoldIsSilentAndAppliesNothing(t *testing.T) {
	t.Parallel()
	rig := &sampleRig{readings: []hostpressure.Reading{{Load1: 17.5, CPUs: 14, MemAvailablePct: 80}}}
	rig.sampler("enforce", 1).sample()
	if len(rig.applied) != 0 || len(rig.events) != 0 {
		t.Fatalf("applied %v events %+v, want neither on a hold", rig.applied, rig.events)
	}
}

func TestShadowStageDoesNotCap_ButSignals(t *testing.T) {
	t.Parallel()
	rig := &sampleRig{readings: []hostpressure.Reading{heavyHost}}
	rig.sampler("shadow", 1).sample()
	if len(rig.applied) != 0 {
		t.Fatalf("applied = %v, want none at shadow", rig.applied)
	}
	if len(rig.events) != 1 {
		t.Fatalf("events = %d, want one for the change", len(rig.events))
	}
	if f := rig.events[0].Fields; f["applied"] != "false" || f["new"] != "6" || f["stage"] != "shadow" {
		t.Errorf("fields = %v, want applied=false new=6", f)
	}
}

func TestHostSampler_ShadowSignalsOnlyOnACapChange(t *testing.T) {
	t.Parallel()
	hold := hostpressure.Reading{Load1: 17.5, CPUs: 14, MemAvailablePct: 80}
	rig := &sampleRig{readings: []hostpressure.Reading{hold, hold, hold, hold, heavyHost}}
	s := rig.sampler("shadow", 1)
	for range 4 {
		s.sample()
	}
	if len(rig.events) != 0 {
		t.Fatalf("events = %d after 4 samples with no change, want 0", len(rig.events))
	}
	s.sample()
	if len(rig.events) != 1 {
		t.Fatalf("events = %d after one change, want 1", len(rig.events))
	}
}

func TestHostSampler_AppendsEveryReadingToTheSamplesFile(t *testing.T) {
	t.Parallel()
	rig := &sampleRig{readings: []hostpressure.Reading{calmHost, heavyHost}}
	s := rig.sampler("shadow", 1)
	s.sample()
	s.sample()
	if len(rig.lines) != 2 {
		t.Fatalf("lines = %d, want one per reading", len(rig.lines))
	}
	var got hostSampleLine
	if err := json.Unmarshal(rig.lines[1], &got); err != nil {
		t.Fatal(err)
	}
	if got.Load1 != 28 || got.CPUs != 14 || got.LoadPerCore != 2 || got.MemAvailablePct != 80 || got.Cap != 12 || got.Decided != 6 || got.Stage != "shadow" || got.TS == "" || got.Score <= 1 {
		t.Errorf("line = %+v", got)
	}
	if rig.lines[1][len(rig.lines[1])-1] != '\n' {
		t.Error("a line must end with a newline")
	}
}

func TestHostSampler_ASamplesWriteErrorWarnsOnce(t *testing.T) {
	t.Parallel()
	rig := &sampleRig{readings: []hostpressure.Reading{calmHost, calmHost, heavyHost}, failLine: errHost}
	s := rig.sampler("enforce", 1)
	for range 3 {
		s.sample()
	}
	if len(rig.warns) != 1 || !errors.Is(rig.warns[0], errHost) {
		t.Fatalf("warns = %v, want one", rig.warns)
	}
	if len(rig.applied) != 1 {
		t.Errorf("applied = %v: a write error must not stop sampling", rig.applied)
	}
}

func TestHostSamplesPath_FollowsTheLogPathOrTheEvolveLogsDir(t *testing.T) {
	t.Parallel()
	if got := hostSamplesPath(loopConfig{EvolveDir: "/e", LogPath: "/e/logs/run1/loop.log"}); got != "/e/logs/run1/host-pressure.ndjson" {
		t.Errorf("with a log path: %s", got)
	}
	if got := hostSamplesPath(loopConfig{EvolveDir: "/e"}); got != "/e/logs/host-pressure.ndjson" {
		t.Errorf("without: %s", got)
	}
}

func TestAppendLine_AppendsAndReportsAnOpenFailure(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "s.ndjson")
	w := appendLine(path)
	if err := w([]byte("a\n")); err != nil {
		t.Fatal(err)
	}
	if err := w([]byte("b\n")); err != nil {
		t.Fatal(err)
	}
	if raw, _ := os.ReadFile(path); string(raw) != "a\nb\n" {
		t.Errorf("file = %q", raw)
	}
	if err := appendLine(filepath.Join(path, "x", "y"))([]byte("c")); err == nil {
		t.Error("want an open error")
	}
}

func TestHostSampler_FailedReadHoldsTheCapAndWarnsBlindOnceAtTheStreak(t *testing.T) {
	t.Parallel()
	rig := &sampleRig{errs: []error{errHost, errHost, errHost, errHost}}
	s := rig.sampler("enforce", 3)
	for range 4 {
		s.sample()
	}
	if len(rig.applied) != 0 {
		t.Fatalf("applied = %v, want none: a failed read holds the cap", rig.applied)
	}
	if len(rig.events) != 1 || rig.events[0].Code != orchestrator.CodeHostBlind || rig.events[0].Fields["os"] != "testos" || rig.events[0].Fields["error"] != errHost.Error() {
		t.Fatalf("events = %+v, want one blind WARN with the error and the os", rig.events)
	}
}

func TestHostSampler_RunSamplesOnEachTickAndStopsWithItsContext(t *testing.T) {
	t.Parallel()
	rig := &sampleRig{readings: []hostpressure.Reading{heavyHost, calmHost}}
	s := rig.sampler("shadow", 1)
	ticks := make(chan time.Time)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.run(ctx, ticks); close(done) }()
	ticks <- time.Time{}
	ticks <- time.Time{}
	cancel()
	<-done
	if rig.calls != 2 {
		t.Fatalf("reads = %d, want 2 (one per tick)", rig.calls)
	}
}

func TestHostConfig_FloorNeverExceedsTheCeiling(t *testing.T) {
	t.Parallel()
	got := hostConfig(adaptiveOn("enforce"), 8, 6)
	if got.Floor != 6 || got.Ceiling != 6 || got.Cooldown != 60*time.Second || got.IncreaseCooldown != 180*time.Second || got.LoadHigh != 1.5 || got.Step != 1 {
		t.Fatalf("config = %+v", got)
	}
}

func TestHostRun_RunsARealCommandAndReportsAFailure(t *testing.T) {
	t.Parallel()
	out, err := hostRun("echo", "hi")
	if err != nil || string(out) != "hi\n" {
		t.Fatalf("out %q err %v", out, err)
	}
	if _, err := hostRun("/nonexistent/binary"); err == nil {
		t.Fatal("want an error from a missing binary")
	}
}

func TestProductionHostReaderAndTickerAreWired(t *testing.T) {
	t.Parallel()
	if productionHostReader() == nil {
		t.Fatal("no production reader")
	}
	ticks, stop := realHostTicker(time.Hour)
	defer stop()
	if ticks == nil {
		t.Fatal("no ticker channel")
	}
}

type hostRig struct {
	b       *loopBatchCoordinator
	dir     string
	ticks   chan time.Time
	reads   chan struct{}
	stopped atomic.Bool
	every   atomic.Int64
	clock   atomic.Int64
}

func newHostRig(t *testing.T, readings ...hostpressure.Reading) *hostRig {
	t.Helper()
	b, dir, _ := supplyCoordinator(t, t.TempDir())
	rig := &hostRig{b: b, dir: dir, ticks: make(chan time.Time, 4), reads: make(chan struct{}, 4)}
	var mu sync.Mutex
	next := 0
	b.hostReader = func() (hostpressure.Reading, error) {
		mu.Lock()
		r := readings[min(next, len(readings)-1)]
		next++
		mu.Unlock()
		rig.reads <- struct{}{}
		return r, nil
	}
	b.hostSamples = func([]byte) error { return nil }
	b.hostTicker = func(every time.Duration) (<-chan time.Time, func()) {
		rig.every.Store(int64(every))
		return rig.ticks, func() { rig.stopped.Store(true) }
	}
	base := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return base.Add(time.Duration(rig.clock.Add(200)) * time.Second) }
	return rig
}

func adaptiveFleet(stage string) policy.FleetConfig {
	oc := enforceOrchestrator
	oc.WindowStart, oc.WindowMin = 4, 1
	oc.Adaptive = adaptiveOn(stage)
	return policy.FleetConfig{Count: 4, Concurrency: 4, PlanSource: "triage", Orchestrator: oc}
}

func (r *hostRig) tickAndAwaitRead(t *testing.T) {
	t.Helper()
	r.ticks <- time.Time{}
	select {
	case <-r.reads:
	case <-time.After(5 * time.Second):
		t.Fatal("the host sampler never read the host")
	}
}

func TestOrchestratedRun_HostPressureShrinksThenRaisesTheWindowAndStopsWithThePool(t *testing.T) {
	t.Parallel()
	rig := newHostRig(t, heavyHost, calmHost)
	launch := func(_ context.Context, _ fleet.CycleSpec) (int, error) {
		rig.tickAndAwaitRead(t)
		rig.tickAndAwaitRead(t)
		return 0, nil
	}
	res := rig.b.orchestratedRun(adaptiveFleet("enforce"), launch, 2)(context.Background(), []fleet.Todo{{ID: "first"}}, noResupply)
	if len(res) != 1 || res[0].ExitCode != 0 {
		t.Fatalf("results = %+v, want the lane to run to exit 0", res)
	}
	if !rig.stopped.Load() || rig.every.Load() != int64(15*time.Second) {
		t.Errorf("ticker stopped %v every %v, want stopped and 15s", rig.stopped.Load(), time.Duration(rig.every.Load()))
	}
	var moves [][2]string
	for _, e := range supplyEvents(t, rig.b, rig.dir, orchestrator.CodeWindow) {
		if e.Fields["cause"] == "host_pressure" {
			moves = append(moves, [2]string{e.Fields["old"], e.Fields["new"]})
		}
	}
	if want := [][2]string{{"4", "2"}, {"2", "3"}}; len(moves) != 2 || moves[0] != want[0] || moves[1] != want[1] {
		t.Fatalf("host_pressure window moves = %v, want %v", moves, want)
	}
	if got := supplyEvents(t, rig.b, rig.dir, orchestrator.CodeHostPressure); len(got) != 2 || got[0].Fields["applied"] != "true" || got[1].Fields["cause"] != "pressure_low" {
		t.Errorf("host pressure events = %+v, want a decrease then an increase, both applied", got)
	}
}

func TestOrchestratedRun_HostPressureAtShadowSignalsAndLeavesTheWindowAlone(t *testing.T) {
	t.Parallel()
	rig := newHostRig(t, heavyHost)
	launch := func(_ context.Context, _ fleet.CycleSpec) (int, error) {
		rig.tickAndAwaitRead(t)
		return 0, nil
	}
	rig.b.orchestratedRun(adaptiveFleet("shadow"), launch, 2)(context.Background(), []fleet.Todo{{ID: "first"}}, noResupply)
	if moves := supplyEvents(t, rig.b, rig.dir, orchestrator.CodeWindow); len(moves) != 0 {
		t.Errorf("window events = %+v, want none at shadow", moves)
	}
	got := supplyEvents(t, rig.b, rig.dir, orchestrator.CodeHostPressure)
	if len(got) != 1 || got[0].Fields["applied"] != "false" || got[0].Fields["new"] != "2" {
		t.Errorf("host pressure events = %+v, want one unapplied decision to 2", got)
	}
}

func TestOrchestratedRun_AbsentAdaptiveStartsNoSamplerAndLeavesCapEqualToSized(t *testing.T) {
	t.Parallel()
	rig := newHostRig(t, heavyHost)
	fc := adaptiveFleet("enforce")
	fc.Orchestrator.Adaptive = policy.AdaptiveConfig{}
	launch := func(_ context.Context, _ fleet.CycleSpec) (int, error) { return 0, nil }
	rig.b.orchestratedRun(fc, launch, 2)(context.Background(), []fleet.Todo{{ID: "first"}}, noResupply)
	if rig.every.Load() != 0 || rig.stopped.Load() {
		t.Error("a sampler started although fleet.orchestrator.window.adaptive is absent")
	}
	if got := supplyEvents(t, rig.b, rig.dir, orchestrator.CodeHostPressure); len(got) != 0 {
		t.Errorf("host pressure events = %+v, want none", got)
	}
}

func TestStartHostPressure_WithoutAWindowStartsNothing(t *testing.T) {
	t.Parallel()
	rig := newHostRig(t, heavyHost)
	stop := rig.b.startHostPressure(context.Background(), adaptiveFleet("enforce"), hostSeat{intake: orchestrator.NewIntake(4)})
	stop()
	if rig.every.Load() != 0 {
		t.Error("a sampler started without a window")
	}
}

func TestStartHostPressure_UsesTheProductionReaderAndTickerByDefault(t *testing.T) {
	t.Parallel()
	b, _, _ := supplyCoordinator(t, t.TempDir())
	fc := adaptiveFleet("shadow")
	fc.Orchestrator.Adaptive.SampleS = 3600
	fc.Concurrency = 0
	w := orchestrator.NewWindow(orchestrator.WindowConfig{Start: 4, Min: 1, Sized: 4}, b.deps.Signals.Emit)
	stop := b.startHostPressure(context.Background(), fc, hostSeat{window: w, intake: orchestrator.NewIntake(4)})
	stop()
}

func TestStartHostPressure_AnAppliedCapWakesThePoolSoARaiseNeverWaitsForALaneExit(t *testing.T) {
	t.Parallel()
	rig := newHostRig(t, heavyHost)
	w := orchestrator.NewWindow(orchestrator.WindowConfig{Start: 4, Min: 1, Sized: 4}, nil)
	intake := orchestrator.NewIntake(4)
	stop := rig.b.startHostPressure(context.Background(), adaptiveFleet("enforce"), hostSeat{window: w, intake: intake})
	rig.tickAndAwaitRead(t)
	stop()
	if w.Size() != 2 {
		t.Fatalf("window = %d, want 2 after the heavy reading", w.Size())
	}
	select {
	case <-intake.Wake():
	default:
		t.Fatal("the pool was not woken after the cap changed")
	}
}

func TestStartHostPressure_ASamplesFileThatCannotBeWrittenWarnsOnTheConsoleAndSamplingGoesOn(t *testing.T) {
	t.Parallel()
	rig := newHostRig(t, heavyHost)
	rig.b.hostSamples = func([]byte) error { return errHost }
	var console lockedBuffer
	rig.b.stderr = &console
	w := orchestrator.NewWindow(orchestrator.WindowConfig{Start: 4, Min: 1, Sized: 4}, nil)
	stop := rig.b.startHostPressure(context.Background(), adaptiveFleet("enforce"), hostSeat{window: w, intake: orchestrator.NewIntake(4)})
	rig.tickAndAwaitRead(t)
	stop()
	if !strings.Contains(console.String(), "host pressure samples file") || w.Size() != 2 {
		t.Fatalf("console = %q window = %d, want the WARN and a window of 2", console.String(), w.Size())
	}
}

func TestStartHostPressure_AnAppliedCapLeavesTheOversightQuotaLimitAlone(t *testing.T) {
	t.Parallel()
	rig := newHostRig(t, heavyHost)
	w := orchestrator.NewWindow(orchestrator.WindowConfig{Start: 4, Min: 1, Sized: 4}, nil)
	intake := orchestrator.NewIntake(4)
	intake.Resize(2)
	stop := rig.b.startHostPressure(context.Background(), adaptiveFleet("enforce"), hostSeat{window: w, intake: intake})
	rig.tickAndAwaitRead(t)
	stop()
	admit := intake.Gate(func(_, _ []fleet.Todo) int { return 0 })
	inFlight := []fleet.Todo{{ID: "a"}, {ID: "b"}}
	if got := admit(inFlight, []fleet.Todo{{ID: "c"}}); got != -1 {
		t.Fatalf("admit with 2 in flight = %d, want -1: the oversight quota limit of 2 must survive a host-pressure apply", got)
	}
}

func TestHostSized_AllThreeSitesUseTheOneSizedWidth(t *testing.T) {
	t.Parallel()
	fc := policy.FleetConfig{Count: 16, Concurrency: 6, PlanSource: "triage", Orchestrator: enforceOrchestrator}
	b, _ := iscsCoordinator(t.TempDir(), t.TempDir(), nil)
	if got, _ := b.oversightResize(fc)(); got != 6 {
		t.Errorf("oversight quota target = %d, want 6", got)
	}
	w := b.laneWindow(fc)
	for range 20 {
		w.CleanLanding()
	}
	if got := w.Size(); got != 6 {
		t.Errorf("lane window sized to %d, want 6", got)
	}
	rig := newHostRig(t, heavyHost)
	fc.Orchestrator.Adaptive = adaptiveOn("shadow")
	stop := rig.b.startHostPressure(context.Background(), fc, hostSeat{window: w, intake: orchestrator.NewIntake(6)})
	rig.tickAndAwaitRead(t)
	stop()
	got := supplyEvents(t, rig.b, rig.dir, orchestrator.CodeHostPressure)
	if len(got) != 1 || got[0].Fields["sized"] != "6" || got[0].Fields["old"] != "6" {
		t.Errorf("controller events = %+v, want sized 6 and old 6", got)
	}
}
