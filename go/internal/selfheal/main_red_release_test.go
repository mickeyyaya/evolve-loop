package selfheal

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var otherKey = baseattr.Key{Package: "example.com/m/other", Test: "TestOther"}

type releaseRun struct {
	consumed   []string
	consumeErr error
}

func (f *boundFixture) release(run *releaseRun, green []baseattr.Key, by string) error {
	f.t.Helper()
	return StopRelease{
		EvolveDir: f.evolveDir, Now: t0.Add(time.Hour),
		Event: signalcenter.Event{Cycle: 7, RunID: "run-7", Phase: "ship", Origin: "the caller"},
		Emit:  func(e signalcenter.Event) { f.events = append(f.events, e) },
		Consume: func(id string) error {
			run.consumed = append(run.consumed, id)
			return run.consumeErr
		},
	}.Keys(green, by)
}

func withSecondKey(s mainstop.Stop) mainstop.Stop {
	s.Keys = append(slices.Clone(s.Keys), otherKey)
	return s
}

func TestReadStop_OnlyAReadableStopThatHoldsHolds(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		prepare func(f *boundFixture)
		holds   bool
		warning string
	}{
		"no record":    {func(*boundFixture) {}, false, ""},
		"a stop holds": {func(f *boundFixture) { f.hold(unchanged) }, true, ""},
		"a stop is released": {func(f *boundFixture) {
			f.hold(func(s mainstop.Stop) mainstop.Stop { return s.Released(s.Keys, "main abc", t0) })
		}, false, ""},
		"a record that does not parse": {func(f *boundFixture) {
			if err := os.WriteFile(mainstop.Path(f.evolveDir), []byte("{not json"), 0o644); err != nil {
				f.t.Fatal(err)
			}
		}, false, "WARN main-red stop: the record could not be read ("},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newBoundFixture(t)
			c.prepare(f)
			var warn strings.Builder

			stop, holds := ReadStop(f.evolveDir, &warn)

			if holds != c.holds || holds != (stop.MainSHA == boundMain && stop.Holds()) {
				t.Errorf("holds = %v, stop = %+v, want holds %v with the stop only when it holds", holds, stop, c.holds)
			}
			if (c.warning == "") != (warn.Len() == 0) || !strings.Contains(warn.String(), c.warning) {
				t.Errorf("warning = %q, want %q", warn.String(), c.warning)
			}
		})
	}
}

func TestStopRelease_TheLastGreenKeyReleasesConsumesAndEmitsOneTime(t *testing.T) {
	t.Parallel()
	f := newBoundFixture(t)
	f.hold(unchanged)
	run := &releaseRun{}

	if err := f.release(run, []baseattr.Key{boundKey}, "batch main abc"); err != nil {
		t.Fatal(err)
	}
	if err := f.release(run, []baseattr.Key{boundKey}, "batch main def"); err != nil {
		t.Fatalf("a release with no stop that holds: %v", err)
	}

	stop := f.stop()
	if stop.Holds() || stop.ReleasedBy != "batch main abc" || stop.ReleasedAt == nil || !stop.ReleasedAt.Equal(t0.Add(time.Hour)) {
		t.Fatalf("stop = %+v, want a release by the first call at the time of the caller", stop)
	}
	if !slices.Equal(run.consumed, []string{boundFixItem}) {
		t.Errorf("consumed = %v, want the fix item one time", run.consumed)
	}
	if len(f.events) != 1 {
		t.Fatalf("%d events, want one: %+v", len(f.events), f.events)
	}
	e := f.events[0]
	if e.Cycle != 7 || e.RunID != "run-7" || e.Phase != "ship" || e.Origin != "the caller" {
		t.Errorf("event = %+v, want the cycle, run, phase and origin of the caller", e)
	}
	if e.Module != signalcenter.ModuleGateRepo || e.Kind != signalcenter.KindGateCorrected || e.Severity != signalcenter.SeverityInfo || e.Code != core.CodeMainRedStopReleased {
		t.Errorf("event = %+v, want an info gate.corrected release of gate.repo", e)
	}
	if !strings.Contains(e.Reason, "released by batch main abc") {
		t.Errorf("reason = %q, want the name of the release", e.Reason)
	}
	want := map[string]string{"main": boundMain, "fix_item": boundFixItem, "released_by": "batch main abc", "parked": "2242, 2249", "fix_item_error": ""}
	for k, v := range want {
		if e.Fields[k] != v {
			t.Errorf("fields[%s] = %q, want %q", k, e.Fields[k], v)
		}
	}
}

func TestStopRelease_AStopWithARedKeyHolds(t *testing.T) {
	t.Parallel()
	cases := map[string][]baseattr.Key{
		"no green key":          nil,
		"one of two keys green": {boundKey},
		"a key not of the stop": {{Package: "example.com/m/none", Test: "TestNone"}},
	}
	for name, green := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newBoundFixture(t)
			f.hold(withSecondKey)
			run := &releaseRun{}

			if err := f.release(run, green, "batch main abc"); err != nil {
				t.Fatal(err)
			}

			if !f.stop().Holds() || len(run.consumed) != 0 || len(f.events) != 0 {
				t.Errorf("stop = %+v, consumed = %v, events = %+v, want a stop that holds and no release", f.stop(), run.consumed, f.events)
			}
			if slices.Contains(f.stop().Keys, boundKey) != (len(green) == 0 || green[0] != boundKey) {
				t.Errorf("keys = %v after green %v, want each green key of the stop removed", f.stop().Keys, green)
			}
		})
	}
}

func TestStopRelease_ANoGreenReleaseDoesNotWriteTheRecord(t *testing.T) {
	t.Parallel()
	f := newBoundFixture(t)
	if err := os.WriteFile(mainstop.Path(f.evolveDir), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := f.release(&releaseRun{}, nil, "batch main abc"); err != nil {
		t.Errorf("a release with no green key read the record: %v", err)
	}
	if err := f.release(&releaseRun{}, []baseattr.Key{boundKey}, "batch main abc"); err == nil {
		t.Error("a release with a green key and a record that does not parse gave no error")
	}
}

func TestStopRelease_AFixItemThatDoesNotMoveMakesTheReleaseAWarning(t *testing.T) {
	t.Parallel()
	f := newBoundFixture(t)
	f.hold(unchanged)
	stuck := errors.New("fix item " + boundFixItem + ": the inbox is not readable")

	err := f.release(&releaseRun{consumeErr: stuck}, []baseattr.Key{boundKey}, "batch main abc")

	if !errors.Is(err, stuck) || f.stop().Holds() {
		t.Fatalf("err = %v, holds = %v, want the stop released and the error of the fix item", err, f.stop().Holds())
	}
	if len(f.events) != 1 || f.events[0].Severity != signalcenter.SeverityWarn || f.events[0].Fields["fix_item_error"] != stuck.Error() {
		t.Errorf("events = %+v, want one warn release with fix_item_error", f.events)
	}
}
