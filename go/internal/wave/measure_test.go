package wave_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
	"github.com/mickeyyaya/evolve-loop/go/internal/wavemeasure"
)

var measureStart = time.Date(2026, 10, 17, 6, 0, 0, 0, time.UTC)

func clock(minutes int) time.Time { return measureStart.Add(time.Duration(minutes) * time.Minute) }

func eventAt(minutes int, kind, code, fields string) string {
	return `{"schema_version":"1.0","seq":1,"pid":1,"ts":"` + clock(minutes).Format(time.RFC3339Nano) +
		`","module":"loop","origin":"x","kind":"` + kind + `","code":"` + code + `","severity":"INFO","reason":"r","fields":{` + fields + `}}` + "\n"
}

func TestReadCycles_ALaneSpansItsSignalsAndItsDossierPhases(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/runs/cycle-11/run.json", `{"cycle_id":11,"phase":"retrospective"}`)
	p.write(".evolve/runs/cycle-11/signals.ndjson",
		eventAt(10, "ledger.appended", "", "")+
			eventAt(30, "ship.error", "SHIP_GIT_FLEET_REBASE_NEEDED", "")+
			eventAt(40, "cycle.sealed", "", `"final_verdict":"PASS"`)+
			eventAt(41, "ledger.appended", "", ""))
	p.write("knowledge-base/cycles/cycle-11.json",
		`{"cycle":11,"final_verdict":"PASS","commit_sha":"abc","phases":[{"name":"scout","verdict":"PASS","started_at":"`+
			clock(5).Format(time.RFC3339)+`","ended_at":"`+clock(15).Format(time.RFC3339)+`"},{"name":"ship","verdict":"PASS","started_at":"bad","ended_at":"`+
			clock(45).Format(time.RFC3339)+`"}]}`)
	p.write(".evolve/runs/cycle-12/run.json", `{"cycle_id":12,"phase":"build"}`)
	p.write(".evolve/runs/cycle-12/signals.ndjson",
		eventAt(20, "ship.error", "SHIP_GIT_PUSH_REJECTED", "")+eventAt(25, "ledger.appended", "", ""))

	got, warnings := wave.ReadCycles(p.root, 10, 0)

	want := []wave.Cycle{
		{ID: 11, Phase: "retrospective", Verdict: "PASS", Shipped: true, Conflict: true, StartedAt: clock(5), EndedAt: clock(45)},
		{ID: 12, Phase: "build", StartedAt: clock(20)},
	}
	if !reflect.DeepEqual(got, want) || len(warnings) != 0 {
		t.Errorf("ReadCycles =\n%+v\nwant\n%+v\nwarnings %q", got, want, warnings)
	}
}

func TestReadCycles_EachLandingConflictCodeMarksTheLane(t *testing.T) {
	t.Parallel()
	for _, code := range []string{"SHIP_GIT_FLEET_REBASE_NEEDED", "SHIP_GIT_FLEET_REBASE_CONFLICT", "SHIP_AUDIT_BINDING_HEAD_MOVED"} {
		p := newPlane(t)
		p.write(".evolve/runs/cycle-3/signals.ndjson", eventAt(1, "ship.error", code, ""))

		got, _ := wave.ReadCycles(p.root, 0, 0)

		if len(got) != 1 || !got[0].Conflict {
			t.Errorf("%s: cycles %+v, want cycle 3 marked as a conflict", code, got)
		}
	}
}

func TestMeasureFacts_ReadsTheSizedWidthAndTheSignalsInsideTheRunWindow(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/signals.ndjson",
		eventAt(-5, "loop.wave", "", `"width_sized":"9"`)+
			eventAt(-1, "loop.wave", wavemeasure.CodeAdmitWait, "")+
			eventAt(2, "loop.wave", "", `"width_sized":"3"`)+
			eventAt(3, "loop.wave", "", `"width_sized":"2"`)+
			eventAt(4, "loop.wave", "", `"width_sized":"x"`)+
			eventAt(5, "loop.wave", wavemeasure.CodeAdmitWait, "")+
			eventAt(6, "loop.wave", wavemeasure.CodeAdmitWait, "")+
			eventAt(7, "loop.wave", "LOOP_ORCH_LANE_DEADLINE", "")+
			eventAt(61, "loop.wave", wavemeasure.CodeAdmitWait, ""))
	rec := wave.Record{Number: 5, StartedAt: clock(0)}
	next := wave.Record{Number: 6, StartedAt: clock(60)}
	cycles := []wave.Cycle{{ID: 7, Shipped: true, Conflict: true, StartedAt: clock(1), EndedAt: clock(50)}, {ID: 8, StartedAt: clock(2)}}

	loop, warnings := wave.ReadLoopSignals(p.root)
	facts, source := wave.MeasureFacts(rec, &next, cycles, loop, 4)

	want := wavemeasure.Facts{
		Start: clock(0), End: clock(60), WidthSized: 2, AdmissionWaits: 2,
		Lanes: []wavemeasure.Lane{{Start: clock(1), End: clock(50), Landed: true, Conflict: true}, {Start: clock(2)}},
	}
	if !reflect.DeepEqual(facts, want) || source != wave.WidthFromSignal || len(warnings) != 0 {
		t.Errorf("MeasureFacts =\n%+v (%s, %q)\nwant\n%+v (%s)", facts, source, warnings, want, wave.WidthFromSignal)
	}
}

func TestMeasureFacts_WithNoSizingRecordThePolicyWidthHolds(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	ended := clock(30)
	rec := wave.Record{Number: 5, StartedAt: clock(0), EndedAt: &ended}
	loop, warnings := wave.ReadLoopSignals(p.root)

	facts, source := wave.MeasureFacts(rec, nil, nil, loop, 4)

	if facts.WidthSized != 4 || source != wave.WidthFromPolicy || !facts.End.Equal(ended) || len(warnings) != 0 || loop != nil {
		t.Errorf("MeasureFacts = %+v (%s, %q), want width 4 from the policy and the recorded end", facts, source, warnings)
	}
}

func TestMeasureFacts_TheLiveRunHasNoEnd(t *testing.T) {
	t.Parallel()
	facts, _ := wave.MeasureFacts(wave.Record{Number: 5, StartedAt: clock(0)}, nil, nil, nil, 4)

	if !facts.End.IsZero() {
		t.Errorf("End = %v, want zero so that the measure reads the clock", facts.End)
	}
}

func TestReadLoopSignals_AMalformedSignalLineIsAWarning(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/signals.ndjson", "not json\n")

	_, warnings := wave.ReadLoopSignals(p.root)

	if len(warnings) != 1 || !strings.Contains(warnings[0], "signals.ndjson") {
		t.Errorf("warnings = %q, want one that names the project signal stream", warnings)
	}
}

func TestReadLoopSignals_AnUnreadableSignalStreamIsAWarning(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/signals.ndjson/x", "")

	_, warnings := wave.ReadLoopSignals(p.root)

	if len(warnings) != 1 || !strings.Contains(warnings[0], "signals.ndjson") {
		t.Errorf("warnings = %q, want one that names the unreadable stream", warnings)
	}
}

func TestReadCycles_ALaneThatRebasedAndThenLandedCountsOneConflict(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/runs/cycle-21/signals.ndjson",
		eventAt(10, "phase.started", "", "")+
			eventAt(20, "ship.error", "SHIP_GIT_FLEET_REBASE_NEEDED", `"class":"transient","stage":"ship"`)+
			eventAt(25, "phase.started", "", "")+
			eventAt(35, "ship.landed", "", "")+
			eventAt(36, "cycle.sealed", "", `"final_verdict":"PASS"`))
	p.write("knowledge-base/cycles/cycle-21.json", `{"cycle":21,"final_verdict":"PASS","commit_sha":"abc"}`)
	cycles, warnings := wave.ReadCycles(p.root, 20, 0)
	rec := wave.Record{Number: 1, StartedAt: clock(0)}

	facts, _ := wave.MeasureFacts(rec, nil, cycles, nil, 1)
	got := wavemeasure.Compute(facts, clock(60))

	if got.Conflicts != 1 || got.Landed != 1 || got.ConflictRate != 1 || len(warnings) != 0 {
		t.Errorf("measures = %+v warnings %q, want 1 conflict of 1 landed for a lane that rebased and landed", got, warnings)
	}
}
