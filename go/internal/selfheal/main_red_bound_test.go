package selfheal

import (
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

const (
	boundMain    = "9a2bbe0e1c0ffee"
	boundFixItem = "main-red-9a2bbe0e1c0f"
)

var boundKey = baseattr.Key{Package: "example.com/m/dup", Test: "TestDup"}

type boundFixture struct {
	t         *testing.T
	evolveDir string
	events    []signalcenter.Event
}

func newBoundFixture(t *testing.T) *boundFixture {
	t.Helper()
	return &boundFixture{t: t, evolveDir: t.TempDir()}
}

func (f *boundFixture) hold(change func(mainstop.Stop) mainstop.Stop) mainstop.Stop {
	f.t.Helper()
	stop, err := mainstop.Update(f.evolveDir, func(s mainstop.Stop, _ bool) (mainstop.Stop, error) {
		opened := s.Opened(baseattr.Verdict{MainRed: []baseattr.Key{boundKey}, Main: boundMain}, boundFixItem, t0.In(time.FixedZone("east", 3600)))
		return change(opened.WithParked(mainstop.Parked{Cycle: 2242}).WithParked(mainstop.Parked{Cycle: 2249})), nil
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return stop
}

func unchanged(s mainstop.Stop) mainstop.Stop { return s }

func (f *boundFixture) check(now time.Time, bound time.Duration) error {
	f.t.Helper()
	return StopBound{EvolveDir: f.evolveDir, Bound: bound, Now: now, Emit: func(e signalcenter.Event) { f.events = append(f.events, e) }}.Check()
}

func (f *boundFixture) stop() mainstop.Stop {
	f.t.Helper()
	stop, _, err := mainstop.Read(f.evolveDir)
	if err != nil {
		f.t.Fatal(err)
	}
	return stop
}

func (f *boundFixture) incidents() []Incident {
	f.t.Helper()
	incidents, err := LoadIncidents(Dir(f.evolveDir))
	if err != nil {
		f.t.Fatal(err)
	}
	return incidents
}

func TestStopBound_AStopPastItsBoundEscalatesOneTime(t *testing.T) {
	t.Parallel()
	f := newBoundFixture(t)
	held := f.hold(unchanged)
	late := t0.Add(2*time.Hour + time.Second)

	if err := f.check(late, 2*time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := f.check(late.Add(time.Hour), 2*time.Hour); err != nil {
		t.Fatal(err)
	}

	stop := f.stop()
	if stop.Escalated == nil || !stop.Escalated.Equal(late) || !stop.Holds() {
		t.Fatalf("stop = %+v, want a stop that holds with escalated_at %v", stop, late)
	}
	incidents := f.incidents()
	if len(incidents) != 1 {
		t.Fatalf("%d incidents, want one: %+v", len(incidents), incidents)
	}
	inc := incidents[0]
	if inc.Class != ClassMainRed || inc.Fingerprint != "main-red:main" || inc.State != StateOpen || inc.Severity != signalcenter.SeverityIncident ||
		inc.Recurrence != 1 || !inc.OpenedAt.Equal(late) || len(inc.Sources) != 1 || inc.Sources[0] != "main-red-stop:"+boundMain {
		t.Errorf("incident = %+v, want one open main-red incident of main with the stop as its source", inc)
	}
	if inc.Summary != held.Cause()+"; to resume: "+held.ResumeCondition() || !strings.Contains(inc.Summary, "2242, 2249") {
		t.Errorf("summary = %q, want the cause %q and the resume condition", inc.Summary, held.Cause())
	}
	if len(f.events) != 1 {
		t.Fatalf("%d signals, want one: %+v", len(f.events), f.events)
	}
	e := f.events[0]
	if e.Code != core.CodeMainRedStopEscalated || e.Kind != signalcenter.KindLoopEscalation || e.Module != signalcenter.ModuleGateRepo ||
		e.Severity != signalcenter.SeverityIncident || e.Origin != "selfheal.StopBound" ||
		e.Cycle != 0 || e.Reason != "main-red stop past its bound of 2h0m0s: "+inc.Summary {
		t.Errorf("signal = %+v, want one incident GATE_REPO_MAIN_RED_STOP_ESCALATED with the bound, the cause and the resume condition", e)
	}
	want := map[string]string{
		"keys": "example.com/m/dup.TestDup", "main": boundMain, "fix_item": boundFixItem, "parked": "2242, 2249",
		"opened_at": "2026-10-27T01:00:00Z", "bound": "2h0m0s", "incident": inc.ID,
	}
	for k, v := range want {
		if e.Fields[k] != v {
			t.Errorf("fields[%s] = %q, want %q", k, e.Fields[k], v)
		}
	}
	if len(e.Fields) != len(want) {
		t.Errorf("fields = %v, want only %v", e.Fields, want)
	}
}

func TestStopBound_AStopThatIsNotDueIsNotEscalated(t *testing.T) {
	t.Parallel()
	escalated := t0.Add(time.Minute)
	cases := []struct {
		name   string
		change func(mainstop.Stop) mainstop.Stop
		now    time.Time
		bound  time.Duration
	}{
		{name: "the stop is at its bound", change: unchanged, now: t0.Add(2 * time.Hour), bound: 2 * time.Hour},
		{name: "the policy gives no bound", change: unchanged, now: t0.Add(72 * time.Hour), bound: 0},
		{name: "the stop is released", change: func(s mainstop.Stop) mainstop.Stop { return s.Released([]baseattr.Key{boundKey}, "cycle 9", t0) }, now: t0.Add(72 * time.Hour), bound: 2 * time.Hour},
		{name: "the stop is escalated already", change: func(s mainstop.Stop) mainstop.Stop { return s.Escalate(escalated) }, now: t0.Add(72 * time.Hour), bound: 2 * time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newBoundFixture(t)
			f.hold(tc.change)

			if err := f.check(tc.now, tc.bound); err != nil {
				t.Fatal(err)
			}

			stop := f.stop()
			if len(f.events) != 0 || len(f.incidents()) != 0 || (stop.Escalated != nil && !stop.Escalated.Equal(escalated)) {
				t.Errorf("signals %d, incidents %d, escalated_at %v; want no escalation", len(f.events), len(f.incidents()), stop.Escalated)
			}
		})
	}
}

func TestStopBound_AnIncidentThatCannotBeWrittenLeavesTheStopForTheNextCheck(t *testing.T) {
	t.Parallel()
	f := newBoundFixture(t)
	f.hold(unchanged)
	if err := os.WriteFile(Dir(f.evolveDir), []byte("a file, not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	late := t0.Add(3 * time.Hour)

	err := f.check(late, 2*time.Hour)

	if err == nil || !strings.Contains(err.Error(), "the escalation of the stop past its bound") {
		t.Fatalf("err = %v, want the escalation error", err)
	}
	if f.stop().Escalated != nil || len(f.events) != 0 {
		t.Fatalf("escalated_at %v, %d signals; want a stop that the next check escalates", f.stop().Escalated, len(f.events))
	}
	if err := os.Remove(Dir(f.evolveDir)); err != nil {
		t.Fatal(err)
	}
	if err := f.check(late, 2*time.Hour); err != nil || len(f.events) != 1 || len(f.incidents()) != 1 {
		t.Errorf("the next check: err %v, %d signals, %d incidents; want one escalation", err, len(f.events), len(f.incidents()))
	}
}

func TestStopBound_ARecordThatCannotBeReadIsAnErrorWithNoIncident(t *testing.T) {
	t.Parallel()
	f := newBoundFixture(t)
	f.hold(unchanged)
	if err := os.WriteFile(mainstop.Path(f.evolveDir), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := f.check(t0.Add(3*time.Hour), 2*time.Hour)

	if err == nil || len(f.events) != 0 || len(f.incidents()) != 0 {
		t.Errorf("err %v, %d signals, %d incidents; want an error and no escalation", err, len(f.events), len(f.incidents()))
	}
}

func TestStopBound_AnOpenMainRedIncidentOfTheCIDetectorIsTheSameIncident(t *testing.T) {
	t.Parallel()
	f := newBoundFixture(t)
	held := f.hold(unchanged)
	seen, older := t0.Add(time.Hour), t0.Add(-24*time.Hour)
	seeded := []Observation{
		{Class: ClassCIHang, Subject: mainBranch, Severity: signalcenter.SeverityWarn, Evidence: []Evidence{{ID: "ci:1", At: older}}, Summary: "a CI run on main ran past the hang limit"},
		{Class: ClassMainRed, Subject: mainBranch, Severity: signalcenter.SeverityIncident, Evidence: []Evidence{{ID: "ci:2", At: seen}}, Summary: "CI is red on main"},
		{Class: ClassCIHang, Subject: "release", Severity: signalcenter.SeverityWarn, Evidence: []Evidence{{ID: "ci:3", At: seen}}, Summary: "a CI run on release ran past the hang limit"},
	}
	if err := UpdateIncidents(Dir(f.evolveDir), func(current []Incident) ([]Incident, error) {
		for _, o := range seeded {
			current, _ = DetectLive(current, o, o.Evidence[0].At)
		}
		return current, nil
	}); err != nil {
		t.Fatal(err)
	}
	before := f.incidents()
	if len(before) != 3 || before[1].Class != ClassMainRed {
		t.Fatalf("seeded incidents = %+v, want the main-red incident between two others", before)
	}

	if err := f.check(t0.Add(3*time.Hour), 90*time.Minute); err != nil {
		t.Fatal(err)
	}

	incidents := f.incidents()
	if len(incidents) != 3 || incidents[1].ID != before[1].ID || incidents[1].Recurrence != 2 || !strings.Contains(incidents[1].Summary, held.ResumeCondition()) {
		t.Fatalf("incidents = %+v, want the incident %s with a second source and the resume condition", incidents, before[1].ID)
	}
	if len(f.events) != 1 || f.events[0].Fields["incident"] != before[1].ID || f.events[0].Fields["bound"] != "1h30m0s" ||
		!strings.HasPrefix(f.events[0].Reason, "main-red stop past its bound of 1h30m0s: ") {
		t.Errorf("signals = %+v, want one that names the incident %s and the bound 1h30m0s", f.events, before[1].ID)
	}
}

func TestStopBound_TheSignalNamesTheMainRedIncidentAndNotAnOlderIncident(t *testing.T) {
	t.Parallel()
	f := newBoundFixture(t)
	f.hold(unchanged)
	hang := Observation{Class: ClassCIHang, Subject: mainBranch, Severity: signalcenter.SeverityWarn, Evidence: []Evidence{{ID: "ci:1", At: t0}}, Summary: "a CI run on main ran past the hang limit"}
	if err := UpdateIncidents(Dir(f.evolveDir), func(current []Incident) ([]Incident, error) {
		next, _ := DetectLive(current, hang, t0)
		return next, nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := f.check(t0.Add(3*time.Hour), 2*time.Hour); err != nil {
		t.Fatal(err)
	}

	incidents := f.incidents()
	red := slices.IndexFunc(incidents, func(inc Incident) bool { return inc.Class == ClassMainRed })
	if len(incidents) != 2 || red < 0 || incidents[1-red].Class != ClassCIHang {
		t.Fatalf("incidents = %+v, want the older ci-hang incident and the main-red incident", incidents)
	}
	if len(f.events) != 1 || f.events[0].Fields["incident"] != incidents[red].ID || incidents[red].ID == incidents[1-red].ID {
		t.Errorf("signals = %+v, want one that names the main-red incident %s", f.events, incidents[red].ID)
	}
}
