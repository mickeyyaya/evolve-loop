package hostsentinel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const gib = 1 << 30

type recorder struct {
	mu       sync.Mutex
	events   []signalcenter.Event
	releases [][]Breach
	applied  []bool
	saved    []Status
	warns    []string
	saveErr  error
	release  []Release
}

func (r *recorder) ports(sample func(context.Context) Sample) Ports {
	return Ports{
		Sample: sample,
		Release: func(_ context.Context, b []Breach, apply bool) []Release {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.releases = append(r.releases, b)
			r.applied = append(r.applied, apply)
			return r.release
		},
		Emit: func(e signalcenter.Event) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.events = append(r.events, e)
		},
		Save: func(s Status) error {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.saved = append(r.saved, s)
			return r.saveErr
		},
		Warn: func(m string) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.warns = append(r.warns, m)
		},
	}
}

func (r *recorder) codes() []signalcenter.Code {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []signalcenter.Code
	for _, e := range r.events {
		out = append(out, e.Code)
	}
	return out
}

func fixed(s Sample) func(context.Context) Sample {
	return func(context.Context) Sample { return s }
}

func testConfig(t *testing.T, stage string) Config {
	t.Helper()
	c, err := Policy{Stage: stage, IntervalSeconds: 60, DiskLowGiB: 20, FloorGiB: 5, TmuxSessionsMax: 10, OrphansMax: 3, LoadPerCPUMax: 2}.Config(100 * gib)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func overEveryThreshold(at time.Time) Sample {
	return Sample{At: at, Readings: map[Metric]Reading{
		MetricDiskFree: {Value: 0.25 * gib},
		MetricGoCache:  {Value: 138 * gib},
		MetricTmux:     {Value: 11},
		MetricOrphans:  {Value: 324},
		MetricLoad:     {Value: 3.5},
	}}
}

func calm(at time.Time) Sample {
	return Sample{At: at, Readings: map[Metric]Reading{
		MetricDiskFree: {Value: 200 * gib},
		MetricGoCache:  {Value: 10 * gib},
		MetricTmux:     {Value: 2},
		MetricOrphans:  {Value: 0},
		MetricLoad:     {Value: 0.5},
	}}
}

func TestPolicyConfig_DefaultsAreShadowEachThreeMinutesWithTheGCCacheCap(t *testing.T) {
	t.Parallel()
	c, err := Policy{}.Config(20 * gib)
	if err != nil {
		t.Fatal(err)
	}
	if c.Stage != StageShadow || c.Interval != 3*time.Minute || c.FloorBytes != 5*gib {
		t.Fatalf("defaults = %+v, want shadow, 3m, 5 GiB floor", c)
	}
	want := map[Metric]float64{MetricDiskFree: 20 * gib, MetricGoCache: 20 * gib, MetricTmux: 40, MetricOrphans: 20, MetricLoad: 2}
	for m, v := range want {
		if c.Limits[m] != v {
			t.Errorf("limit %s = %v, want %v", m, c.Limits[m], v)
		}
	}
}

func TestPolicyConfig_ReadsEachStageAndRefusesAnUnknownOne(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]Stage{"off": StageOff, "shadow": StageShadow, "enforce": StageEnforce, "": StageShadow} {
		c, err := Policy{Stage: in}.Config(gib)
		if err != nil || c.Stage != want {
			t.Errorf("stage %q = %q, %v; want %q", in, c.Stage, err, want)
		}
	}
	if _, err := (Policy{Stage: "loud"}).Config(gib); err == nil || !strings.Contains(err.Error(), "loud") {
		t.Fatalf("an unknown stage must be refused with its name, got %v", err)
	}
}

func TestEvaluate_EachMetricOverItsThresholdIsOneBreachWithTheValueAndTheThreshold(t *testing.T) {
	t.Parallel()
	got := Evaluate(overEveryThreshold(time.Unix(0, 0)), testConfig(t, "shadow"))
	want := []Breach{
		{Code: CodeDiskLow, Metric: MetricDiskFree, Value: 0.25 * gib, Threshold: 20 * gib},
		{Code: CodeCacheHigh, Metric: MetricGoCache, Value: 138 * gib, Threshold: 100 * gib},
		{Code: CodeTmuxHigh, Metric: MetricTmux, Value: 11, Threshold: 10},
		{Code: CodeOrphansHigh, Metric: MetricOrphans, Value: 324, Threshold: 3},
		{Code: CodeLoadHigh, Metric: MetricLoad, Value: 3.5, Threshold: 2},
	}
	if len(got) != len(want) {
		t.Fatalf("Evaluate = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("breach %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestEvaluate_AFailedReadingAZeroThresholdAndACalmHostBreachNothing(t *testing.T) {
	t.Parallel()
	c := testConfig(t, "shadow")
	if got := Evaluate(calm(time.Unix(0, 0)), c); len(got) != 0 {
		t.Fatalf("a calm host breached %+v", got)
	}
	failed := Sample{Readings: map[Metric]Reading{MetricDiskFree: Measured(0, errors.New("statfs failed"))}}
	if got := Evaluate(failed, c); len(got) != 0 {
		t.Fatalf("a failed disk reading is not a proven low disk, got %+v", got)
	}
	c.Limits[MetricTmux] = 0
	if got := Evaluate(Sample{Readings: map[Metric]Reading{MetricTmux: {Value: 99}}}, c); len(got) != 0 {
		t.Fatalf("a zero threshold turns the check off, got %+v", got)
	}
}

func TestMeasured_KeepsTheValueOrTheError(t *testing.T) {
	t.Parallel()
	if r := Measured(7, nil); r.Value != 7 || r.Err != "" {
		t.Fatalf("Measured(7, nil) = %+v", r)
	}
	if r := Measured(7, errors.New("boom")); r.Value != 0 || r.Err != "boom" {
		t.Fatalf("Measured(7, boom) = %+v", r)
	}
}

func TestTick_OverEachThresholdEmitsOneHostSignalAndARepeatInTheIntervalDoesNotFlood(t *testing.T) {
	t.Parallel()
	var r recorder
	start := time.Unix(1_000_000, 0)
	at := start
	s := New(testConfig(t, "shadow"), r.ports(func(context.Context) Sample { return overEveryThreshold(at) }))
	s.Tick(context.Background())
	first := r.codes()
	want := []signalcenter.Code{CodeDiskLow, CodeCacheHigh, CodeTmuxHigh, CodeOrphansHigh, CodeLoadHigh, CodeFloorHold}
	if strings.Join(codeStrings(first), ",") != strings.Join(codeStrings(want), ",") {
		t.Fatalf("first tick emitted %v, want %v", first, want)
	}
	e := r.events[0]
	if e.Module != signalcenter.ModuleHost || e.Severity != signalcenter.SeverityWarn || e.Fields["value"] != "268435456" || e.Fields["threshold"] != "21474836480" || e.Fields["stage"] != "shadow" {
		t.Fatalf("disk signal = %+v, want module host, WARN, the value and the threshold", e)
	}
	at = start.Add(30 * time.Second)
	s.Tick(context.Background())
	if n := len(r.codes()); n != len(want) {
		t.Fatalf("a repeat within the interval emitted again: %v", r.codes())
	}
	at = start.Add(57 * time.Second)
	s.Tick(context.Background())
	if n := len(r.codes()); n != 2*len(want) {
		t.Fatalf("a breach that lasts past the interval must signal again, got %v", r.codes())
	}
}

func codeStrings(cs []signalcenter.Code) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = string(c)
	}
	return out
}

func TestTick_AtShadowAsksOnlyForThePreviewAndHoldsNoLane(t *testing.T) {
	t.Parallel()
	r := recorder{release: []Release{{Scope: "cache", Report: []string{"evolve gc --dry-run: 3 go build cache file(s) would be trimmed"}}}}
	s := New(testConfig(t, "shadow"), r.ports(fixed(overEveryThreshold(time.Unix(5, 0)))))
	st := s.Tick(context.Background())
	if len(r.applied) != 1 || r.applied[0] {
		t.Fatalf("shadow must ask for a preview only, apply calls = %v", r.applied)
	}
	if st.HoldLanes || s.HoldLanes() {
		t.Fatal("shadow must hold no lane below the floor")
	}
	if r.events[0].Fields["released"] != "cache=would" {
		t.Fatalf("the signal must name what it would release, got %q", r.events[0].Fields["released"])
	}
	if !hasCode(r.codes(), CodeFloorHold) || floorEvent(&r).Fields["held"] != "false" {
		t.Fatalf("below the floor at shadow the sentinel names the hold it would make, events %+v", r.events)
	}
}

func TestTick_AtEnforceAppliesTheReleaseAndBelowTheFloorHoldsNewLanes(t *testing.T) {
	t.Parallel()
	r := recorder{release: []Release{{Scope: "cache", Applied: true}, {Scope: "orphans", LeaseHeld: true}}}
	at := time.Unix(5, 0)
	sample := overEveryThreshold(at)
	s := New(testConfig(t, "enforce"), r.ports(func(context.Context) Sample { return sample }))
	st := s.Tick(context.Background())
	if len(r.applied) != 1 || !r.applied[0] || len(r.releases[0]) != 5 {
		t.Fatalf("enforce must apply the release for the breaches, calls %v %v", r.applied, r.releases)
	}
	if !st.HoldLanes || !s.HoldLanes() || floorEvent(&r).Fields["held"] != "true" {
		t.Fatal("below the floor at enforce no new lane may start")
	}
	if got := r.events[0].Fields["released"]; got != "cache=applied,orphans=lease_held" {
		t.Fatalf("released field = %q", got)
	}
	sample = calm(at.Add(time.Hour))
	s.Tick(context.Background())
	if s.HoldLanes() {
		t.Fatal("above the floor again, new lanes start again")
	}
	if len(r.applied) != 1 {
		t.Fatal("a calm host asks for no release")
	}
}

func TestTick_AReleaseThatFailsIsNamedInTheSignal(t *testing.T) {
	t.Parallel()
	r := recorder{release: []Release{{Scope: "cache"}}}
	s := New(testConfig(t, "enforce"), r.ports(fixed(overEveryThreshold(time.Unix(5, 0)))))
	s.Tick(context.Background())
	if got := r.events[0].Fields["released"]; got != "cache=failed" {
		t.Fatalf("released field = %q, want cache=failed", got)
	}
}

func TestTick_AFailedDiskReadingKeepsTheFloorHold(t *testing.T) {
	t.Parallel()
	var r recorder
	sample := overEveryThreshold(time.Unix(5, 0))
	s := New(testConfig(t, "enforce"), r.ports(func(context.Context) Sample { return sample }))
	s.Tick(context.Background())
	sample = Sample{At: time.Unix(500, 0), Readings: map[Metric]Reading{MetricDiskFree: Measured(0, errors.New("statfs failed"))}}
	if st := s.Tick(context.Background()); !st.HoldLanes || !s.HoldLanes() {
		t.Fatal("a failed disk reading does not prove free disk, so the hold stays")
	}
}

func TestHoldLanes_ANilSentinelHoldsNothing(t *testing.T) {
	t.Parallel()
	var s *Sentinel
	if s.HoldLanes() {
		t.Fatal("no sentinel holds no lane")
	}
}

func hasCode(cs []signalcenter.Code, c signalcenter.Code) bool {
	for _, x := range cs {
		if x == c {
			return true
		}
	}
	return false
}

func floorEvent(r *recorder) signalcenter.Event {
	for _, e := range r.events {
		if e.Code == CodeFloorHold {
			return e
		}
	}
	return signalcenter.Event{}
}

func TestTick_AFailedSampleWarnsOnceAndAFailedSaveWarnsWithoutAStop(t *testing.T) {
	t.Parallel()
	r := recorder{saveErr: errors.New("disk full")}
	sample := Sample{At: time.Unix(9, 0), Readings: map[Metric]Reading{
		MetricLoad:     Measured(0, errors.New("no /proc/loadavg")),
		MetricDiskFree: {Value: 200 * gib},
	}}
	s := New(testConfig(t, "enforce"), r.ports(fixed(sample)))
	st := s.Tick(context.Background())
	s.Tick(context.Background())
	if len(st.Breaches) != 0 || st.HoldLanes {
		t.Fatalf("a failed reading is no breach and no hold: %+v", st)
	}
	if n := strings.Count(strings.Join(codeStrings(r.codes()), ","), string(CodeSampleFailed)); n != 1 {
		t.Fatalf("a failed sample must signal once per interval, got %v", r.codes())
	}
	if e := r.events[0]; e.Fields["metrics"] != "load_per_cpu" || !strings.Contains(e.Reason, "no /proc/loadavg") {
		t.Fatalf("sample-failed signal = %+v", e)
	}
	joined := strings.Join(r.warns, "\n")
	if !strings.Contains(joined, "load_per_cpu") || !strings.Contains(joined, "disk full") {
		t.Fatalf("warnings = %q, want the failed metric and the failed save", joined)
	}
}

func TestStart_TicksEachIntervalAndStopLeavesNoTicker(t *testing.T) {
	t.Parallel()
	var ticks atomic.Int32
	var r recorder
	c := testConfig(t, "shadow")
	c.Interval = 5 * time.Millisecond
	ports := r.ports(func(context.Context) Sample {
		ticks.Add(1)
		return calm(time.Now())
	})
	stop := New(c, ports).Start(context.Background())
	deadline := time.Now().Add(5 * time.Second)
	for ticks.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	stop()
	after := ticks.Load()
	if after < 2 {
		t.Fatalf("the sentinel ticked %d time(s) in 5s at a 5ms interval", after)
	}
	time.Sleep(30 * time.Millisecond)
	if ticks.Load() != after {
		t.Fatal("the sentinel ticked after stop returned")
	}
}

func TestStart_TheLoopContextEndStopsTheSentinel(t *testing.T) {
	t.Parallel()
	var r recorder
	c := testConfig(t, "shadow")
	c.Interval = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	stop := New(c, r.ports(fixed(calm(time.Now())))).Start(ctx)
	cancel()
	done := make(chan struct{})
	go func() { stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not return after the loop context ended")
	}
	if len(r.saved) != 0 {
		t.Fatal("the first sample waits one interval")
	}
}

func TestParseLoadavg_DividesTheOneMinuteLoadByTheCPUs(t *testing.T) {
	t.Parallel()
	v, err := ParseLoadavg([]byte("8.00 4.00 2.00 3/900 12345\n"), 4)
	if err != nil || v != 2 {
		t.Fatalf("ParseLoadavg = %v, %v; want 2", v, err)
	}
	for _, bad := range []string{"", "x 1 1"} {
		if _, err := ParseLoadavg([]byte(bad), 4); err == nil {
			t.Errorf("ParseLoadavg(%q) must fail", bad)
		}
	}
	if _, err := ParseLoadavg([]byte("1 1 1"), 0); err == nil {
		t.Error("zero CPUs must fail")
	}
}

func TestReadStatus_RoundTripsTheLastSampleAndAMissingFileIsNoStatus(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st, err := ReadStatus(dir)
	if err != nil || st != nil {
		t.Fatalf("no file = %v, %v; want nil, nil", st, err)
	}
	want := Status{Stage: StageShadow, Sample: calm(time.Unix(100, 0).UTC())}
	if err := WriteStatus(dir, want); err != nil {
		t.Fatal(err)
	}
	st, err = ReadStatus(dir)
	if err != nil || st == nil || !st.Sample.At.Equal(want.Sample.At) || st.Sample.Readings[MetricTmux].Value != 2 {
		t.Fatalf("ReadStatus = %+v, %v", st, err)
	}
	if err := os.WriteFile(filepath.Join(dir, StatusFile), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadStatus(dir); err == nil {
		t.Fatal("a torn status file must be an error")
	}
	if err := WriteStatus(filepath.Join(dir, StatusFile, "x"), want); err == nil {
		t.Fatal("a write under a file must fail")
	}
	if _, err := ReadStatus(filepath.Join(dir, StatusFile, "x")); err == nil {
		t.Fatal("a read under a file must fail")
	}
}

func TestStatusSummary_NamesEachReadingTheBreachesAndTheHold(t *testing.T) {
	t.Parallel()
	s := Status{Stage: StageEnforce, Sample: overEveryThreshold(time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)), HoldLanes: true,
		Breaches: []Breach{{Code: CodeDiskLow}, {Code: CodeOrphansHigh}}}
	s.Sample.Readings[MetricTmux] = Measured(0, errors.New("no tmux"))
	got := s.Summary()
	for _, want := range []string{"2026-11-04T09:00:00Z", "stage enforce", "disk free 0.27 GB", "go cache 148.18 GB", "tmux failed", "orphans 324", "load per cpu 3.50", "HOST_DISK_LOW,HOST_ORPHANS_HIGH", "new lanes held"} {
		if !strings.Contains(got, want) {
			t.Errorf("Summary() = %q, want %q", got, want)
		}
	}
	partial := calm(time.Unix(0, 0))
	delete(partial.Readings, MetricLoad)
	calmSummary := Status{Stage: StageShadow, Sample: partial}.Summary()
	if strings.Contains(calmSummary, "breaches") || strings.Contains(calmSummary, "held") || strings.Contains(calmSummary, "load") {
		t.Errorf("a calm summary names no breach and no hold: %q", calmSummary)
	}
}
