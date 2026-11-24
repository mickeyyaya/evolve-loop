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
	"github.com/mickeyyaya/evolve-loop/go/internal/hostsentinel"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const sentinelGB = uint64(1e9)

type sentinelRig struct {
	b         *loopBatchCoordinator
	dir       string
	ticks     chan time.Time
	recorded  chan []byte
	reads     chan bool
	stopped   atomic.Bool
	every     atomic.Int64
	mu        sync.Mutex
	samples   []hostsentinel.Sample
	next      int
	readErr   error
	events    []signalcenter.Event
	releases  atomic.Int32
	releaseFn func(ctx context.Context) midWaveOutcome
	clock     atomic.Int64
}

func newSentinelRig(t *testing.T, holdStage, releaseStage string, samples ...hostsentinel.Sample) *sentinelRig {
	t.Helper()
	b, dir, _ := supplyCoordinator(t, t.TempDir())
	rig := &sentinelRig{b: b, dir: dir, ticks: make(chan time.Time, 8), recorded: make(chan []byte, 64), reads: make(chan bool, 64), samples: samples}
	policyJSON := `{"gc":{"disk_floor_gb":10,"disk_warn_gb":20,"go_cache_max_gb":20,"sentinel":{"hold":{"stage":"` + holdStage + `"},"release":{"stage":"` + releaseStage + `"}}}}`
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "policy.json"), []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	rig.releaseFn = func(context.Context) midWaveOutcome { return midWaveOutcome{} }
	b.deps.Signals.Subscribe(func(e signalcenter.Event) {
		rig.mu.Lock()
		defer rig.mu.Unlock()
		rig.events = append(rig.events, e)
	})
	b.sentinel = sentinelPorts{
		read: func(_ context.Context, full bool) (hostsentinel.Sample, error) {
			rig.mu.Lock()
			defer rig.mu.Unlock()
			rig.reads <- full
			if rig.readErr != nil {
				return hostsentinel.Sample{}, rig.readErr
			}
			s := rig.samples[min(rig.next, len(rig.samples)-1)]
			rig.next++
			return s, nil
		},
		release: func(ctx context.Context) midWaveOutcome {
			rig.releases.Add(1)
			return rig.releaseFn(ctx)
		},
		ticker: func(every time.Duration) (<-chan time.Time, func()) {
			rig.every.Store(int64(every))
			return rig.ticks, func() { rig.stopped.Store(true) }
		},
		record: func(line []byte) error {
			rig.recorded <- line
			return nil
		},
	}
	base := time.Date(2026, 11, 21, 8, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return base.Add(time.Duration(rig.clock.Load()) * time.Second) }
	return rig
}

func (r *sentinelRig) advance(d time.Duration) { r.clock.Add(int64(d.Seconds())) }

func (r *sentinelRig) tick(t *testing.T) []byte {
	t.Helper()
	r.ticks <- time.Time{}
	select {
	case line := <-r.recorded:
		return line
	case <-time.After(5 * time.Second):
		t.Fatal("the sentinel never recorded a sample")
		return nil
	}
}

func (r *sentinelRig) codes(t *testing.T, code signalcenter.Code) []signalcenter.Event {
	t.Helper()
	r.b.deps.Signals.Flush()
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []signalcenter.Event
	for _, e := range r.events {
		if e.Code == code {
			out = append(out, e)
		}
	}
	return out
}

func sample(freeGB uint64) hostsentinel.Sample {
	return hostsentinel.Sample{FreeBytes: freeGB * sentinelGB, Path: "/p"}
}

func (r *sentinelRig) start(t *testing.T) (*fleet.FloorHold, func()) {
	t.Helper()
	hold, stop := r.b.startSentinel(context.Background())
	t.Cleanup(stop)
	<-r.recorded
	return hold, stop
}

func TestSentinel_ShadowStageReleasesNothingAndRecordsWhatItWouldRelease(t *testing.T) {
	t.Parallel()
	over := sample(15)
	over.CacheBytes, over.CacheRead = int64(30*sentinelGB), true
	rig := newSentinelRig(t, "shadow", "shadow", over)
	hold, _ := rig.start(t)
	rig.tick(t)
	if n := rig.releases.Load(); n != 0 {
		t.Fatalf("shadow ran the release %d times", n)
	}
	got := rig.codes(t, hostsentinel.CodeWouldRelease)
	if len(got) != 1 || got[0].Fields["scopes"] != "go_cache,pipeline_temp" {
		t.Fatalf("would-release events = %+v, want one naming the scopes", got)
	}
	if hold.Held() {
		t.Fatal("shadow set the hold")
	}
}

func TestSentinel_ShadowRecordsAWouldHoldAndSetsNoHold(t *testing.T) {
	t.Parallel()
	rig := newSentinelRig(t, "shadow", "shadow", sample(5))
	hold, _ := rig.start(t)
	got := rig.codes(t, hostsentinel.CodeWouldHold)
	if len(got) != 1 || got[0].Fields["would_hold"] != "true" || hold.Held() {
		t.Fatalf("would-hold events %+v held %v, want one would_hold=true and no hold", got, hold.Held())
	}
	if lane := rig.codes(t, hostsentinel.CodeLaneHold); len(lane) != 0 {
		t.Fatalf("shadow emitted %d lane holds", len(lane))
	}
}

func TestSentinel_AHeldGCLeaseSkipsOnceAndTheNextTickInsideRepeatEmitsNone(t *testing.T) {
	t.Parallel()
	low := sample(15)
	rig := newSentinelRig(t, "shadow", "enforce", low)
	rig.releaseFn = func(context.Context) midWaveOutcome { return midWaveOutcome{Skipped: "gc_lease_held"} }
	rig.start(t)
	rig.advance(time.Second)
	rig.tick(t)
	rig.advance(time.Minute)
	rig.tick(t)
	if n := rig.releases.Load(); n != 1 {
		t.Fatalf("release attempts = %d, want 1 inside repeat_s", n)
	}
	got := rig.codes(t, hostsentinel.CodeReleaseSkipped)
	if len(got) != 1 || got[0].Fields["reason"] != "gc_lease_held" {
		t.Fatalf("skip events = %+v, want exactly one", got)
	}
}

func TestSentinel_AReleaseThatRanReportsFreedAndRemeasuredBytes(t *testing.T) {
	t.Parallel()
	rig := newSentinelRig(t, "shadow", "enforce", sample(15))
	rig.releaseFn = func(context.Context) midWaveOutcome {
		return midWaveOutcome{Freed: 7_000_000_000, After: hostsentinel.Sample{FreeBytes: 22 * sentinelGB, CacheBytes: 53_000_000_000, CacheRead: true}}
	}
	rig.start(t)
	rig.tick(t)
	got := rig.codes(t, hostsentinel.CodeReleased)
	if len(got) != 1 || got[0].Fields["freed_bytes"] != "7000000000" || got[0].Fields["cache_bytes"] != "53000000000" {
		t.Fatalf("released events = %+v", got)
	}
}

func TestSentinel_AFailedReleaseIsReportedAsASkipWithTheFailure(t *testing.T) {
	t.Parallel()
	rig := newSentinelRig(t, "shadow", "enforce", sample(15))
	rig.releaseFn = func(context.Context) midWaveOutcome { return midWaveOutcome{Err: "the cache scope failed: boom"} }
	rig.start(t)
	rig.tick(t)
	got := rig.codes(t, hostsentinel.CodeReleaseSkipped)
	if len(got) != 1 || !strings.Contains(got[0].Fields["reason"], "boom") {
		t.Fatalf("skip events = %+v, want the failure named", got)
	}
}

func TestSentinel_ANoProgressReleaseStartsNoSecondRunUntilTheReadingGrows(t *testing.T) {
	t.Parallel()
	over := sample(100)
	over.CacheBytes, over.CacheRead = int64(30*sentinelGB), true
	rig := newSentinelRig(t, "shadow", "enforce", over)
	rig.releaseFn = func(context.Context) midWaveOutcome {
		return midWaveOutcome{Freed: 100, After: over}
	}
	rig.start(t)
	rig.tick(t)
	if n := rig.releases.Load(); n != 1 {
		t.Fatalf("releases = %d, want the first run only", n)
	}
	rig.advance(time.Hour)
	rig.tick(t)
	if n := rig.releases.Load(); n != 1 {
		t.Fatalf("releases = %d, want no second run for a reading that did not grow", n)
	}
	grown := over
	grown.CacheBytes = int64(31 * sentinelGB)
	rig.mu.Lock()
	rig.samples = []hostsentinel.Sample{grown}
	rig.next = 0
	rig.mu.Unlock()
	rig.advance(time.Hour)
	rig.tick(t)
	if n := rig.releases.Load(); n != 2 {
		t.Fatalf("releases = %d, want a second run once the reading grew", n)
	}
}

func TestSentinel_DiskUnderTheFloorHoldsAndAboveWarnClears(t *testing.T) {
	t.Parallel()
	rig := newSentinelRig(t, "enforce", "shadow", hostsentinel.Sample{FreeBytes: 10*sentinelGB - 1, Path: "/p"})
	hold, _ := rig.start(t)
	if !hold.Held() {
		t.Fatal("floor-1 did not set the hold")
	}
	<-hold.Wake()
	rig.setSample(sample(15))
	rig.advance(time.Minute)
	rig.tick(t)
	if !hold.Held() {
		t.Fatal("a disk between the floor and warn cleared the hold")
	}
	select {
	case <-hold.Wake():
	case <-time.After(time.Second):
		t.Fatal("a held tick did not poke the pool")
	}
	rig.setSample(sample(20))
	rig.advance(time.Minute)
	rig.tick(t)
	if hold.Held() {
		t.Fatal("a disk at warn did not clear the hold")
	}
	select {
	case <-hold.Wake():
	case <-time.After(time.Second):
		t.Fatal("the clear did not wake the pool")
	}
	if got := rig.codes(t, hostsentinel.CodeLaneHold); len(got) != 1 || got[0].Kind != signalcenter.KindLoopWave {
		t.Fatalf("lane hold events = %+v, want one", got)
	}
}

func TestSentinel_AHoldPastHoldMaxEmitsOneStuckIncidentAndKeepsTheHold(t *testing.T) {
	t.Parallel()
	rig := newSentinelRig(t, "enforce", "shadow", sample(5))
	hold, _ := rig.start(t)
	rig.advance(2 * time.Hour)
	rig.tick(t)
	rig.advance(time.Hour)
	rig.tick(t)
	got := rig.codes(t, hostsentinel.CodeLaneHoldStuck)
	if len(got) != 1 || got[0].Severity != signalcenter.SeverityIncident || !hold.Held() {
		t.Fatalf("stuck events %+v held %v, want one incident and the hold kept", got, hold.Held())
	}
}

func (r *sentinelRig) setSample(s hostsentinel.Sample) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.samples, r.next = []hostsentinel.Sample{s}, 0
}

func TestSentinel_TheFirstSampleReadsOnlyFreeDiskAndHoldsBeforeTheFirstLaunch(t *testing.T) {
	t.Parallel()
	rig := newSentinelRig(t, "enforce", "shadow", sample(5))
	hold, _ := rig.start(t)
	if full := <-rig.reads; full {
		t.Fatal("the first sample was a full reading")
	}
	if !hold.Held() {
		t.Fatal("the hold was not set before the runner started")
	}
	launched := make(chan string, 1)
	launch := func(_ context.Context, spec fleet.CycleSpec) (int, error) {
		launched <- spec.Scope[0]
		return 0, nil
	}
	rig.b.floorHold = hold
	done := make(chan []fleet.Result, 1)
	run, _ := rig.b.poolRunner(policy.FleetConfig{Count: 2, PlanSource: "triage", Scheduling: "pool"}, launch, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { done <- run(ctx, []fleet.Todo{{ID: "A"}}, nil) }()
	select {
	case id := <-launched:
		t.Fatalf("lane %s launched while held", id)
	case <-time.After(150 * time.Millisecond):
	}
	cancel()
	<-done
}

func TestSentinel_EveryTenthTickIsAFullReading(t *testing.T) {
	t.Parallel()
	rig := newSentinelRig(t, "shadow", "shadow", sample(100))
	rig.start(t)
	<-rig.reads
	var fulls []bool
	for i := 0; i < 10; i++ {
		rig.tick(t)
		fulls = append(fulls, <-rig.reads)
	}
	want := []bool{false, false, false, false, false, false, false, false, false, true}
	for i := range want {
		if fulls[i] != want[i] {
			t.Fatalf("reads after tick %d full=%v, want pattern %v", i, fulls[i], want)
		}
	}
}

func TestSentinel_AFailedSampleWarnsOnceAndTheNextGoodSampleActs(t *testing.T) {
	t.Parallel()
	rig := newSentinelRig(t, "enforce", "shadow", sample(100))
	_, console := rig.b.stderr, rig.b.stderr.(*lockedBuffer)
	hold, _ := rig.start(t)
	<-rig.reads
	rig.mu.Lock()
	rig.readErr = errors.New("statfs failed")
	rig.mu.Unlock()
	for i := 0; i < 5; i++ {
		rig.ticks <- time.Time{}
		<-rig.reads
	}
	rig.mu.Lock()
	rig.readErr = nil
	rig.mu.Unlock()
	rig.setSample(sample(5))
	rig.tick(t)
	if got := rig.codes(t, hostsentinel.CodeBlind); len(got) != 1 || !strings.Contains(got[0].Fields["reason"], "statfs failed") {
		t.Fatalf("blind events = %+v, want exactly one", got)
	}
	if n := strings.Count(console.String(), "[loop] WARN: the host sentinel cannot read the host"); n != 1 {
		t.Fatalf("warnings = %d, want one:\n%s", n, console.String())
	}
	if !hold.Held() {
		t.Fatal("the next good sample did not act")
	}
}

func (r *sentinelRig) failReads(t *testing.T, n int) {
	t.Helper()
	for drained := false; !drained; {
		select {
		case <-r.reads:
		default:
			drained = true
		}
	}
	r.mu.Lock()
	r.readErr = errors.New("statfs failed")
	r.mu.Unlock()
	for i := 0; i < n; i++ {
		r.ticks <- time.Time{}
		<-r.reads
	}
	r.mu.Lock()
	r.readErr = nil
	r.mu.Unlock()
}

func TestSentinel_TheBlindSignalComesAtTheThirdFailureInARowAndAgainAfterEachRecovery(t *testing.T) {
	t.Parallel()
	rig := newSentinelRig(t, "shadow", "shadow", sample(100))
	_, console := rig.b.stderr, rig.b.stderr.(*lockedBuffer)
	rig.start(t)
	<-rig.reads
	streaks := []struct{ failures, wantBlind int }{
		{hostsentinel.BlindAfter - 1, 0},
		{hostsentinel.BlindAfter, 1},
		{hostsentinel.BlindAfter + 2, 2},
	}
	for _, st := range streaks {
		rig.failReads(t, st.failures)
		rig.tick(t)
		if got := rig.codes(t, hostsentinel.CodeBlind); len(got) != st.wantBlind {
			t.Fatalf("after %d failures and a good sample: blind events = %d, want %d", st.failures, len(got), st.wantBlind)
		}
	}
	if n := strings.Count(console.String(), "[loop] WARN: the host sentinel cannot read the host"); n != 2 {
		t.Fatalf("warnings = %d, want one for each blind streak:\n%s", n, console.String())
	}
}

func TestSentinel_TheFirstSampleDefersAnEnforcedReleaseToTheFirstTick(t *testing.T) {
	t.Parallel()
	rig := newSentinelRig(t, "shadow", "enforce", sample(15))
	rig.start(t)
	if n := rig.releases.Load(); n != 0 {
		t.Fatalf("the first sample ran %d releases before the pool started, want none", n)
	}
	rig.tick(t)
	if n := rig.releases.Load(); n != 1 {
		t.Fatalf("releases after the first tick = %d, want 1", n)
	}
}

func TestSentinel_StopsWithTheLoopAndLeavesNoTask(t *testing.T) {
	t.Parallel()
	over := sample(15)
	rig := newSentinelRig(t, "shadow", "enforce", over)
	childSaw := make(chan struct{})
	started := make(chan struct{})
	rig.releaseFn = func(ctx context.Context) midWaveOutcome {
		started <- struct{}{}
		<-ctx.Done()
		close(childSaw)
		return midWaveOutcome{Err: "stopped"}
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, stop := rig.b.startSentinel(ctx)
	rig.ticks <- time.Time{}
	<-started
	done := make(chan struct{})
	go func() {
		cancel()
		stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not return")
	}
	select {
	case <-childSaw:
	default:
		t.Fatal("the in-flight release never saw its context cancel")
	}
	if !rig.stopped.Load() {
		t.Fatal("the ticker was not stopped")
	}
}

func TestSentinel_AStopFunctionAloneEndsTheTask(t *testing.T) {
	t.Parallel()
	rig := newSentinelRig(t, "shadow", "shadow", sample(100))
	_, stop := rig.b.startSentinel(context.Background())
	stop()
	if !rig.stopped.Load() {
		t.Fatal("the ticker was not stopped")
	}
}

func TestSentinel_TheTickerUsesTheIntervalOfThePolicy(t *testing.T) {
	t.Parallel()
	rig := newSentinelRig(t, "shadow", "shadow", sample(100))
	rig.start(t)
	if got := time.Duration(rig.every.Load()); got != 60*time.Second {
		t.Fatalf("interval = %v, want the 60 s default", got)
	}
}

func TestSentinelNdjsonPath_FollowsTheHostSamplesPathRule(t *testing.T) {
	t.Parallel()
	cfg := loopConfig{EvolveDir: "/plane/.evolve"}
	if got, want := sentinelSamplesPath(cfg), filepath.Join("/plane/.evolve", "logs", "host-sentinel.ndjson"); got != want {
		t.Errorf("without --log: %q, want %q", got, want)
	}
	cfg.LogPath = "/runs/loop/run.log"
	if got, want := sentinelSamplesPath(cfg), filepath.Join("/runs/loop", "host-sentinel.ndjson"); got != want {
		t.Errorf("with --log: %q, want %q", got, want)
	}
	if filepath.Dir(sentinelSamplesPath(cfg)) != filepath.Dir(hostSamplesPath(cfg)) {
		t.Error("the sentinel file and the host pressure file sit in different dirs")
	}
}

func TestSentinel_TheRecordLineCarriesTheSampleAndTheHoldState(t *testing.T) {
	t.Parallel()
	s := sample(5)
	s.CacheBytes, s.CacheRead, s.Servers, s.Orphans, s.CountsRead = 41e9, true, 3, 4, true
	rig := newSentinelRig(t, "enforce", "shadow", s)
	rig.start(t)
	line := rig.tick(t)
	var rec sentinelLine
	if err := json.Unmarshal(line, &rec); err != nil {
		t.Fatalf("record %q: %v", line, err)
	}
	if rec.FreeBytes != 5*sentinelGB || rec.CacheBytes != 41e9 || rec.Servers != 3 || rec.Orphans != 4 || !rec.Held || rec.HoldStage != "enforce" || rec.ReleaseStage != "shadow" || rec.TS == "" {
		t.Fatalf("record = %+v", rec)
	}
}

func TestSentinel_AFailedRecordWarnsOnce(t *testing.T) {
	t.Parallel()
	rig := newSentinelRig(t, "shadow", "shadow", sample(100))
	recorded := make(chan struct{}, 8)
	rig.b.sentinel.record = func([]byte) error {
		recorded <- struct{}{}
		return errors.New("disk full")
	}
	_, stop := rig.b.startSentinel(context.Background())
	t.Cleanup(stop)
	<-recorded
	for i := 0; i < 3; i++ {
		rig.ticks <- time.Time{}
		<-recorded
	}
	stop()
	console := rig.b.stderr.(*lockedBuffer)
	if n := strings.Count(console.String(), "disk full"); n != 1 {
		t.Fatalf("record warnings = %d, want one:\n%s", n, console.String())
	}
}

func TestSentinelConfig_ReadsTheThresholdsOfTheGCPolicy(t *testing.T) {
	t.Parallel()
	var p policy.Policy
	if err := json.Unmarshal([]byte(`{"gc":{"disk_floor_gb":25,"disk_warn_gb":40,"go_cache_max_gb":30,"sentinel":{"orphan_servers_max":3,"orphan_processes_max":4,"repeat_s":5,"hold_max_s":6,"no_progress_bytes":7}}}`), &p); err != nil {
		t.Fatal(err)
	}
	got := sentinelConfig(gcBlock(p))
	want := hostsentinel.Config{FloorBytes: 25 * sentinelGB, WarnBytes: 40 * sentinelGB, CacheLimitBytes: int64(30 * sentinelGB), ServersMax: 3, OrphansMax: 4,
		Repeat: 5 * time.Second, HoldMax: 6 * time.Second, NoProgressBytes: 7}
	if got != want {
		t.Fatalf("config = %+v, want %+v", got, want)
	}
}

func TestSentinel_RunsUnderBothPoolRunnersAndAHoldStopsNewLanesInEach(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"shadow", "enforce"} {
		t.Run("orchestrator "+stage, func(t *testing.T) {
			t.Parallel()
			rig := newSentinelRig(t, "enforce", "shadow", sample(5))
			hold, _ := rig.start(t)
			rig.b.floorHold = hold
			fc := adaptiveFleet("shadow")
			fc.Orchestrator.Stage = stage
			launched := make(chan string, 4)
			launch := func(_ context.Context, spec fleet.CycleSpec) (int, error) {
				launched <- spec.Scope[0]
				return 0, nil
			}
			run, mode := rig.b.poolRunner(fc, launch, 2)
			wantMode := map[string]string{"shadow": "rolling", "enforce": "orchestrator"}[stage]
			if mode != wantMode {
				t.Fatalf("mode = %q, want %q", mode, wantMode)
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan []fleet.Result, 1)
			go func() { done <- run(ctx, []fleet.Todo{{ID: "A"}}, noResupply) }()
			select {
			case id := <-launched:
				t.Fatalf("%s runner launched %s while held", mode, id)
			case <-time.After(150 * time.Millisecond):
			}
			cancel()
			<-done
		})
	}
}

type readyHold struct {
	fleet.Hold
	ready chan struct{}
	once  sync.Once
}

func (h *readyHold) Wake() <-chan struct{} {
	h.once.Do(func() { close(h.ready) })
	return h.Hold.Wake()
}

func TestPool_AClearWakesThePoolUnderBothRunners(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"shadow", "enforce"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			rig := newSentinelRig(t, "enforce", "shadow", sample(5))
			hold, _ := rig.start(t)
			spy := &readyHold{Hold: hold, ready: make(chan struct{})}
			rig.b.floorHold = spy
			fc := adaptiveFleet("shadow")
			fc.Orchestrator.Stage = stage
			launched := make(chan string, 4)
			launch := func(_ context.Context, spec fleet.CycleSpec) (int, error) {
				launched <- spec.Scope[0]
				return 0, nil
			}
			run, _ := rig.b.poolRunner(fc, launch, 2)
			done := make(chan []fleet.Result, 1)
			go func() { done <- run(context.Background(), []fleet.Todo{{ID: "A"}}, noResupply) }()
			<-spy.ready
			rig.setSample(sample(50))
			rig.advance(time.Minute)
			rig.tick(t)
			select {
			case id := <-launched:
				if id != "A" {
					t.Fatalf("launched %s", id)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("the clear did not start a lane")
			}
			<-done
		})
	}
}

func TestPool_ABrakeEngagedWhileHeldWithNoLaneReturnsAfterTheNextTick(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"shadow", "enforce"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			rig := newSentinelRig(t, "enforce", "shadow", sample(5))
			hold, _ := rig.start(t)
			rig.b.floorHold = hold
			fc := adaptiveFleet("shadow")
			fc.Orchestrator.Stage = stage
			launched := make(chan string, 4)
			launch := func(_ context.Context, spec fleet.CycleSpec) (int, error) {
				launched <- spec.Scope[0]
				return 0, nil
			}
			run, _ := rig.b.poolRunner(fc, launch, 2)
			done := make(chan []fleet.Result, 1)
			go func() { done <- run(context.Background(), []fleet.Todo{{ID: "A"}}, noResupply) }()
			select {
			case <-done:
				t.Fatal("the held pool returned before the brake")
			case <-time.After(100 * time.Millisecond):
			}
			if err := os.WriteFile(paths.LoopStopPath(rig.dir), nil, 0o644); err != nil {
				t.Fatal(err)
			}
			rig.tick(t)
			select {
			case res := <-done:
				if len(res) != 1 || res[0].Err != fleet.ErrNeverLaunched {
					t.Fatalf("results = %+v", res)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("the brake did not end the held pool after the next tick")
			}
			select {
			case id := <-launched:
				t.Fatalf("launched %s under the brake", id)
			default:
			}
		})
	}
}

func TestSentinel_CUJ_ThroughTheRealLoopEntry(t *testing.T) {
	t.Parallel()
	low := sample(8)
	low.CacheBytes, low.CacheRead = int64(41*sentinelGB), true
	rig := newSentinelRig(t, "shadow", "shadow", low)
	rig.b.cfg.GoalHash = "sentinel-cuj"
	rig.b.deps.Storage = &erroringReadStorage{}
	out := rig.b.dispatchPool(1, poolPreflightFleet, rig.b.cycleLaunch("/nonexistent-binary", 0))
	if out.route == routePool {
		t.Fatalf("setup: the pool ran: %+v", out)
	}
	if got := rig.codes(t, hostsentinel.CodeDiskFloor); len(got) != 1 || got[0].Fields["free_bytes"] != "8000000000" || got[0].Fields["floor_bytes"] != "10000000000" {
		t.Fatalf("disk floor events through the real signal center = %+v", got)
	}
	if got := rig.codes(t, hostsentinel.CodeWouldHold); len(got) != 1 {
		t.Fatalf("would-hold events = %+v", got)
	}
	if !rig.stopped.Load() {
		t.Fatal("the sentinel outlived the pool dispatch")
	}
}
