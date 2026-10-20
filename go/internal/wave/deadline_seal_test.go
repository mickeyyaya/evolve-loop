package wave_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func TestReadRunCycles_TheLoopSealHoldsAndTheDeadlineSignalIsTheFallback(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	stop := func(minutes int, todo string) string {
		return eventAt(minutes, "loop.wave", "LOOP_ORCH_LANE_DEADLINE", `"action":"stopped","todo":"`+todo+`"`)
	}
	p.write(".evolve/signals.ndjson", stop(30, "unsealed")+stop(30, "sealed"))
	p.write(".evolve/runs/cycle-7/lane-scope.json", `{"todo_ids":["unsealed"]}`)
	p.write(".evolve/runs/cycle-7/signals.ndjson", eventAt(1, "ledger.appended", "", ""))
	p.write(".evolve/runs/cycle-8/lane-scope.json", `{"todo_ids":["sealed"]}`)
	p.write(".evolve/runs/cycle-8/signals.ndjson", eventAt(2, "ledger.appended", "", "")+
		eventAt(32, "cycle.sealed", "", `"termination_reason":"`+cyclestate.CycleTerminationLaneDeadline+`"`))

	cycles, loop, warnings := wave.ReadRunCycles(p.root, 6, 0)

	want := []wave.Cycle{
		{ID: 7, Todo: "unsealed", Termination: cyclestate.CycleTerminationLaneDeadline, StartedAt: clock(1), EndedAt: clock(30)},
		{ID: 8, Todo: "sealed", Termination: cyclestate.CycleTerminationLaneDeadline, StartedAt: clock(2), EndedAt: clock(32)},
	}
	if !reflect.DeepEqual(cycles, want) || len(loop) != 2 || len(warnings) != 0 {
		t.Errorf("ReadRunCycles =\n%+v\nwant\n%+v\nloop %d events, warnings %q", cycles, want, len(loop), warnings)
	}
}

func TestReadRunCycles_JoinsTheWarningsOfBothStreams(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/signals.ndjson", "not json\n")
	p.write(".evolve/runs/cycle-7/signals.ndjson", "not json\n")

	_, _, warnings := wave.ReadRunCycles(p.root, 6, 0)

	if len(warnings) != 2 {
		t.Errorf("warnings = %q, want one for the cycle stream and one for the loop stream", warnings)
	}
}

func TestApplyLaneDeadlines_ASealedNewestCycleKeepsTheStopOffAnOlderCycle(t *testing.T) {
	t.Parallel()
	stop := poolEvent(30, "LOOP_ORCH_LANE_DEADLINE", "x")
	stop.Fields["action"] = "stopped"
	cycles := []wave.Cycle{
		{ID: 1, Todo: "x", Phase: "build", StartedAt: clock(0)},
		{ID: 2, Todo: "x", Verdict: "PASS", StartedAt: clock(5), EndedAt: clock(28)},
	}

	got := wave.ApplyLaneDeadlines(cycles, []signalcenter.Event{stop})

	if !reflect.DeepEqual(got, cycles) {
		t.Errorf("ApplyLaneDeadlines =\n%+v\nwant no change: the stop belongs to the newest cycle of x, which has its own seal", got)
	}
}

func TestReadCycles_ALoopSealAfterARealSealKeepsTheVerdict(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/runs/cycle-9/signals.ndjson", eventAt(1, "ledger.appended", "", "")+
		eventAt(20, "cycle.sealed", "", `"final_verdict":"PASS"`)+
		eventAt(21, "cycle.sealed", "", `"termination_reason":"`+cyclestate.CycleTerminationLaneDeadline+`"`))

	got, _ := wave.ReadCycles(p.root, 8, 0)

	if len(got) != 1 || got[0].Verdict != "PASS" || got[0].Termination != "" || !got[0].EndedAt.Equal(clock(20)) {
		t.Errorf("cycles = %+v, want the PASS seal of the lane to hold over the later loop seal", got)
	}
}
