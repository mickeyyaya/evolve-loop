package wave_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
	"github.com/mickeyyaya/evolve-loop/go/internal/wavemeasure"
)

func poolEvent(minutes int, code, todo string) signalcenter.Event {
	return signalcenter.Event{TS: clock(minutes).Format("2006-01-02T15:04:05Z07:00"), Kind: signalcenter.KindLoopWave, Code: signalcenter.Code(code), Fields: map[string]string{"todo": todo}}
}

func TestMeasureFacts_ThePoolSignalsGiveTheLaneSpans(t *testing.T) {
	t.Parallel()
	loop := []signalcenter.Event{
		poolEvent(-5, "LOOP_ORCH_LANE_LAUNCHED", "old"),
		poolEvent(1, "LOOP_ORCH_LANE_LAUNCHED", "a"),
		poolEvent(2, "LOOP_ORCH_LANE_LAUNCHED", "b"),
		poolEvent(3, "LOOP_ORCH_LANE_EXITED", "unknown"),
		poolEvent(10, "LOOP_ORCH_LANE_EXITED", "a"),
		poolEvent(10, "LOOP_ORCH_LANE_BACKFILLED", "a"),
		poolEvent(20, "LOOP_ORCH_LANE_EXITED", "a"),
		poolEvent(70, "LOOP_ORCH_LANE_EXITED", "b"),
	}
	rec := wave.Record{Number: 5, StartedAt: clock(0)}
	next := wave.Record{Number: 6, StartedAt: clock(60)}
	cycles := []wave.Cycle{{ID: 7, Shipped: true, Conflict: true, StartedAt: clock(1)}, {ID: 8, StartedAt: clock(2)}}

	facts, _ := wave.MeasureFacts(rec, &next, cycles, loop, 4)

	want := []wavemeasure.Lane{
		{Landed: true, Conflict: true}, {},
		{Start: clock(1), End: clock(10)}, {Start: clock(2)}, {Start: clock(10), End: clock(20)},
	}
	if !reflect.DeepEqual(facts.Lanes, want) {
		t.Errorf("lanes =\n%+v\nwant\n%+v", facts.Lanes, want)
	}
}

func TestApplyLaneDeadlines_AStoppedLaneStopsTheCycleThatRanIt(t *testing.T) {
	t.Parallel()
	deadline := func(minutes int, todo, action string) signalcenter.Event {
		e := poolEvent(minutes, "LOOP_ORCH_LANE_DEADLINE", todo)
		e.Fields["action"] = action
		return e
	}
	cycles := []wave.Cycle{
		{ID: 1, Todo: "a", Phase: "build", StartedAt: clock(0)},
		{ID: 2, Todo: "a", Phase: "build", StartedAt: clock(5)},
		{ID: 3, Todo: "a", Phase: "build", StartedAt: clock(40)},
		{ID: 4, Todo: "b", Phase: "build", StartedAt: clock(0)},
		{ID: 5, Todo: "c", Verdict: "PASS", StartedAt: clock(0)},
		{ID: 6, Todo: "a", Phase: "scout"},
	}
	loop := []signalcenter.Event{
		deadline(30, "a", "stopped"),
		deadline(30, "b", "recorded"),
		deadline(30, "c", "stopped"),
		deadline(30, "missing", "stopped"),
		poolEvent(31, "LOOP_ORCH_LANE_EXITED", "a"),
	}

	got := wave.ApplyLaneDeadlines(cycles, loop)

	want := []wave.Cycle{
		{ID: 1, Todo: "a", Phase: "build", StartedAt: clock(0)},
		{ID: 2, Todo: "a", Phase: "build", Termination: wave.TerminationLaneDeadline, StartedAt: clock(5), EndedAt: clock(30)},
		{ID: 3, Todo: "a", Phase: "build", StartedAt: clock(40)},
		{ID: 4, Todo: "b", Phase: "build", StartedAt: clock(0)},
		{ID: 5, Todo: "c", Verdict: "PASS", StartedAt: clock(0)},
		{ID: 6, Todo: "a", Phase: "scout"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ApplyLaneDeadlines =\n%+v\nwant\n%+v", got, want)
	}
	if cycles[1].Termination != "" {
		t.Errorf("the input cycle changed: %+v", cycles[1])
	}
}

func TestReadCycles_TheLaneScopeNamesTheCycleTodo(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/runs/cycle-4/run.json", `{"cycle_id":4,"phase":"build"}`)
	p.write(".evolve/runs/cycle-4/lane-scope.json", `{"todo_ids":["lane-a"]}`)

	got, _ := wave.ReadCycles(p.root, 0, 0)

	if len(got) != 1 || got[0].Todo != "lane-a" {
		t.Errorf("cycles = %+v, want cycle 4 with todo lane-a", got)
	}
}
