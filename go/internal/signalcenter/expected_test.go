package signalcenter

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (f *fakeClock) now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

func (f *fakeClock) advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = f.t.Add(d)
}

var testPairs = []ExpectedPair{{Name: "wave", Start: KindWaveStarted, Ends: []Kind{KindWaveEnded}, DeadlineKey: "expected_signal.wave_seconds", Key: keyByRunID}}

func testDeadlines(key string) (time.Duration, bool) {
	if key == "expected_signal.wave_seconds" {
		return 10 * time.Minute, true
	}
	return 0, false
}

func startEvent(kind Kind, run string) Event {
	return Event{Module: ModuleLoop, Origin: "Test.emit", Kind: kind, Severity: SeverityInfo, Reason: "r", Fields: map[string]string{"run_id": run}}
}

type streamRig struct {
	clock *fakeClock
	path  string
	out   *Center
}

func newStreamRig(t *testing.T) *streamRig {
	t.Helper()
	r := &streamRig{clock: &fakeClock{t: time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)}, path: filepath.Join(t.TempDir(), StreamFileName)}
	r.out = New(WithClock(r.clock.now))
	r.out.Subscribe(r.out.NDJSONSink(func(int) string { return r.path }))
	return r
}

func (r *streamRig) boot(pairs []ExpectedPair) (*ExpectedSignals, *Center, *[]Event) {
	c := New(WithClock(r.clock.now))
	c.Subscribe(c.NDJSONSink(func(int) string { return r.path }))
	got, _ := collect(c)
	return NewExpectedSignals(c, pairs, testDeadlines, r.clock.now), c, got
}

func collect(c *Center) (*[]Event, func()) {
	var mu sync.Mutex
	var got []Event
	unsub := c.Subscribe(func(e Event) {
		if e.Kind == KindExpectedAbsent {
			mu.Lock()
			got = append(got, e)
			mu.Unlock()
		}
	})
	return &got, unsub
}

func TestExpectedSignal_PollWarnsOnceAtTheDeadlineWithTheStartsCycle(t *testing.T) {
	t.Parallel()
	r := newStreamRig(t)
	x, _, got := r.boot(testPairs)
	start := startEvent(KindWaveStarted, "w1")
	start.Cycle = 7
	r.out.Emit(start)
	r.clock.advance(9 * time.Minute)
	if err := x.Poll(r.path); err != nil || len(*got) != 0 {
		t.Fatalf("before the deadline there is no warning: %v %+v", err, *got)
	}
	r.clock.advance(2 * time.Minute)
	if err := x.Poll(r.path); err != nil || len(*got) != 1 {
		t.Fatalf("after the deadline one warning: %v %+v", err, *got)
	}
	e := (*got)[0]
	if e.Code != CodeExpectedAbsent || e.Severity != SeverityWarn || e.Module != ModuleSignal || e.Fields["pair"] != "wave" || e.Cycle != 7 {
		t.Errorf("absent signal = %+v", e)
	}
	r.clock.advance(time.Hour)
	_ = x.Poll(r.path)
	if len(*got) != 1 {
		t.Errorf("the same start warns once, got %d", len(*got))
	}
}

func TestExpectedSignal_DeadlineBoundaryWarnsAtExactlyTheLimit(t *testing.T) {
	t.Parallel()
	r := newStreamRig(t)
	x, _, got := r.boot(testPairs)
	r.out.Emit(startEvent(KindWaveStarted, "w1"))
	r.clock.advance(10*time.Minute - time.Nanosecond)
	_ = x.Poll(r.path)
	if len(*got) != 0 {
		t.Fatalf("one tick short of the limit gives no warning: %+v", *got)
	}
	r.clock.advance(time.Nanosecond)
	_ = x.Poll(r.path)
	if len(*got) != 1 {
		t.Errorf("the clock at exactly the limit gives one warning: %+v", *got)
	}
}

func TestExpectedSignal_TwoBootsOverOneStreamWarnExactlyOnce(t *testing.T) {
	t.Parallel()
	r := newStreamRig(t)
	r.out.Emit(startEvent(KindWaveStarted, "w1"))
	r.clock.advance(time.Hour)
	first, _, firstGot := r.boot(testPairs)
	if err := first.Poll(r.path); err != nil {
		t.Fatal(err)
	}
	second, _, secondGot := r.boot(testPairs)
	if err := second.Poll(r.path); err != nil {
		t.Fatal(err)
	}
	if len(*firstGot)+len(*secondGot) != 1 {
		t.Errorf("warnings across two boots = %d + %d, want 1", len(*firstGot), len(*secondGot))
	}
}

func TestExpectedSignal_TwoWavesStartAndEndInProductionOrderWithoutAFalseWarning(t *testing.T) {
	t.Parallel()
	r := newStreamRig(t)
	x, _, got := r.boot(testPairs)
	r.out.Emit(startEvent(KindWaveStarted, "w1"))
	r.out.Emit(startEvent(KindWaveStarted, "w2"))
	r.out.Emit(startEvent(KindWaveEnded, "w1"))
	r.out.Emit(startEvent(KindWaveEnded, "w2"))
	r.clock.advance(time.Hour)
	_ = x.Poll(r.path)
	if len(*got) != 0 {
		t.Errorf("both waves ended, got %+v", *got)
	}
}

func TestExpectedSignal_EndBeforeTheDeadlineClosesOnlyItsStart(t *testing.T) {
	t.Parallel()
	r := newStreamRig(t)
	x, _, got := r.boot(testPairs)
	r.out.Emit(startEvent(KindWaveStarted, "w1"))
	r.out.Emit(startEvent(KindWaveStarted, "w2"))
	r.out.Emit(startEvent(KindWaveEnded, "w1"))
	r.clock.advance(time.Hour)
	_ = x.Poll(r.path)
	if len(*got) != 1 || (*got)[0].Fields["key"] != "wave|w2" {
		t.Errorf("only the wave without an end warns, got %+v", *got)
	}
}

func TestExpectedSignal_PairWithoutADeadlineIsOff(t *testing.T) {
	t.Parallel()
	r := newStreamRig(t)
	pairs := []ExpectedPair{{Name: "x", Start: KindWaveStarted, Ends: []Kind{KindWaveEnded}, DeadlineKey: "missing", Key: keyByRunID}}
	x, _, got := r.boot(pairs)
	r.out.Emit(startEvent(KindWaveStarted, "w1"))
	r.clock.advance(24 * time.Hour)
	_ = x.Poll(r.path)
	if len(*got) != 0 {
		t.Errorf("a pair with no deadline never warns, got %+v", *got)
	}
}

func TestExpectedSignal_PollOfAMissingStreamIsQuietAndOtherFaultsSurface(t *testing.T) {
	t.Parallel()
	x := NewExpectedSignals(New(), testPairs, testDeadlines, time.Now)
	if err := x.Poll(filepath.Join(t.TempDir(), "absent.ndjson")); err != nil {
		t.Errorf("a stream not yet written is not a fault: %v", err)
	}
	if err := x.Poll(t.TempDir()); err == nil {
		t.Error("an unreadable stream is an error the caller can see")
	}
}

func TestExpectedSignal_PollReadsOnlyNewBytes(t *testing.T) {
	t.Parallel()
	r := newStreamRig(t)
	x, _, got := r.boot(testPairs)
	r.out.Emit(startEvent(KindWaveStarted, "w1"))
	_ = x.Poll(r.path)
	r.out.Emit(startEvent(KindWaveEnded, "w1"))
	_ = x.Poll(r.path)
	r.clock.advance(time.Hour)
	_ = x.Poll(r.path)
	if len(*got) != 0 || len(x.open) != 0 {
		t.Errorf("open=%+v got=%+v", x.open, *got)
	}
}

func phaseEvent(kind Kind, cycle int, run, phase string, fields map[string]string) Event {
	origin := OriginPhaseOutcome
	if kind == KindPhaseDispatched {
		origin = OriginPhaseDispatched
	}
	return Event{Module: ModuleOrchestrator, Origin: origin, Kind: kind, Severity: SeverityInfo, Reason: "r", Cycle: cycle, RunID: run, Phase: phase, Fields: fields}
}

func TestExpectedSignal_PhaseStartFilterKeepsCycleZeroOut(t *testing.T) {
	t.Parallel()
	r := newStreamRig(t)
	x, _, got := r.boot(DefaultExpectedPairs())
	x.deadline = func(string) (time.Duration, bool) { return time.Hour, true }
	r.out.Emit(phaseEvent(KindPhaseDispatched, 0, "", "build", nil))
	r.clock.advance(100 * time.Hour)
	_ = x.Poll(r.path)
	if len(*got) != 0 || len(x.open) != 0 {
		t.Fatalf("a cycle-0 phase opens nothing: open=%+v got=%+v", x.open, *got)
	}
	r.out.Emit(phaseEvent(KindPhaseDispatched, 3, "run", "build", nil))
	r.clock.advance(2 * time.Hour)
	_ = x.Poll(r.path)
	if len(*got) != 1 || (*got)[0].Cycle != 3 || (*got)[0].Fields["end_kind"] != "phase.outcome,phase.aborted" {
		t.Errorf("a cycle phase warns: %+v", *got)
	}
}

func TestExpectedSignal_ProductionShapedPhaseEventsGiveNoWarning(t *testing.T) {
	t.Parallel()
	r := newStreamRig(t)
	x, _, got := r.boot(DefaultExpectedPairs())
	x.deadline = func(string) (time.Duration, bool) { return time.Hour, true }
	r.out.Emit(phaseEvent(KindPhaseDispatched, 5, "run-a", "build", nil))
	r.out.Emit(phaseEvent(KindPhaseOutcome, 5, "", "build", map[string]string{"verdict": "PASS", "archetype": "builder", "duration_ms": "9"}))
	r.out.Emit(phaseEvent(KindPhaseDispatched, 5, "run-a", "audit", nil))
	r.out.Emit(phaseEvent(KindPhaseOutcome, 5, "other-run", "audit", map[string]string{"verdict": "PASS"}))
	r.clock.advance(100 * time.Hour)
	_ = x.Poll(r.path)
	if len(*got) != 0 {
		t.Errorf("dispatch and outcome match on cycle and phase: %+v", *got)
	}
}

func TestExpectedSignal_AnAbortedPhaseClosesItsStart(t *testing.T) {
	t.Parallel()
	r := newStreamRig(t)
	x, _, got := r.boot(DefaultExpectedPairs())
	x.deadline = func(string) (time.Duration, bool) { return time.Hour, true }
	r.out.Emit(phaseEvent(KindPhaseDispatched, 5, "run-a", "build", nil))
	aborted := phaseEvent(KindPhaseAborted, 5, "run-a", "build", map[string]string{"verdict": "FAIL", "abort_reason": "budget"})
	aborted.Severity, aborted.Code = SeverityWarn, "ORCHESTRATOR_PHASE_ABORTED"
	r.out.Emit(aborted)
	r.clock.advance(100 * time.Hour)
	_ = x.Poll(r.path)
	if len(*got) != 0 {
		t.Errorf("an aborted phase is an end: %+v", *got)
	}
}

func TestExpectedPairs_KeysSeparateWhatTheyShouldAndJoinWhatTheyShould(t *testing.T) {
	t.Parallel()
	if keyByRunID(Event{Fields: map[string]string{"run_id": "w"}, Cycle: 4}) != "w" {
		t.Error("the wave key is the run id alone")
	}
	a, b := keyByCyclePhase(Event{Cycle: 1, Phase: "build", RunID: "x"}), keyByCyclePhase(Event{Cycle: 1, Phase: "build", RunID: "y"})
	if a != b || a == keyByCyclePhase(Event{Cycle: 2, Phase: "build"}) || a == keyByCyclePhase(Event{Cycle: 1, Phase: "audit"}) {
		t.Errorf("the phase key is cycle and phase: %q %q", a, b)
	}
}

func TestExpectedSignal_UnparsableTimestampFallsBackToTheClock(t *testing.T) {
	t.Parallel()
	clock := &fakeClock{t: time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)}
	x := NewExpectedSignals(New(), testPairs, testDeadlines, clock.now)
	x.track(Event{Kind: KindWaveStarted, TS: "bad", Fields: map[string]string{"run_id": "w1"}})
	if len(x.open) != 1 || !x.open["wave|w1"].started.Equal(clock.now()) {
		t.Errorf("open = %+v", x.open)
	}
}

func TestExpectedPairs_DefaultsNameTheirDeadlineKeys(t *testing.T) {
	t.Parallel()
	for _, p := range DefaultExpectedPairs() {
		if p.Name == "" || !p.Start.Known() || len(p.Ends) == 0 || p.Key == nil || p.DeadlineKey == "" {
			t.Errorf("incomplete default pair %+v", p)
		}
	}
}

func TestExpectedKinds_AreTheStartsEndsAndTheAbsentWarning(t *testing.T) {
	t.Parallel()
	kinds := ExpectedKinds()
	want := []Kind{KindWaveStarted, KindWaveEnded, KindPhaseDispatched, KindPhaseOutcome, KindPhaseAborted, KindExpectedAbsent}
	if len(kinds) != len(want) {
		t.Errorf("kinds = %v", kinds)
	}
	for _, k := range want {
		if !kinds[k] {
			t.Errorf("%s missing", k)
		}
	}
}

func TestExpectedAbsent_IsRegisteredWithADoc(t *testing.T) {
	t.Parallel()
	owner, ok := IsRegistered(CodeExpectedAbsent)
	if !ok || owner != ModuleSignal || string(CodeExpectedAbsent) != "SIGNAL_EXPECTED_ABSENT" {
		t.Errorf("owner=%q registered=%v", owner, ok)
	}
	if KindLoopTick.Terminal() || KindExpectedAbsent.Terminal() || !KindLoopTick.Known() || string(KindLoopTick) != "loop.tick" {
		t.Error("loop.tick is a known, non-terminal kind")
	}
}

func TestExpectedSignal_OnlyTheOrchestratorsDispatchAndClosingOutcomeCount(t *testing.T) {
	t.Parallel()
	r := newStreamRig(t)
	x, _, got := r.boot(DefaultExpectedPairs())
	x.deadline = func(string) (time.Duration, bool) { return time.Hour, true }
	cloud := phaseEvent(KindPhaseDispatched, 6, "run-a", "build", nil)
	cloud.Module, cloud.Origin = ModuleBridge, "claudeCloud.launch"
	r.out.Emit(cloud)
	r.out.Emit(phaseEvent(KindPhaseDispatched, 5, "run-a", "build", nil))
	selfCheck := phaseEvent(KindPhaseOutcome, 5, "run-a", "build", map[string]string{"decision": "delivered"})
	selfCheck.Module, selfCheck.Origin = ModuleBuild, "builderSelfCheck.record"
	r.out.Emit(selfCheck)
	r.clock.advance(2 * time.Hour)
	_ = x.Poll(r.path)
	if len(*got) != 1 || (*got)[0].Cycle != 5 {
		t.Errorf("a mid-phase outcome must not close the start and a non-orchestrator dispatch must not open one: %+v", *got)
	}
}
