package orchestrator

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func todosNamed(ids ...string) []fleet.Todo {
	out := make([]fleet.Todo, len(ids))
	for i, id := range ids {
		out[i] = fleet.Todo{ID: id}
	}
	return out
}

func liveFields(events *eventLog, code signalcenter.Code) []string {
	var out []string
	for _, e := range events.withCode(code) {
		out = append(out, e.Fields["live"])
	}
	return out
}

func startN(t *testing.T, g *gatedLaunch, n int) []string {
	t.Helper()
	ids := make([]string, n)
	for i := range ids {
		ids[i] = waitStart(t, g)
	}
	return ids
}

func finishAll(t *testing.T, g *gatedLaunch, done chan []fleet.Result, ids ...string) {
	t.Helper()
	for _, id := range ids {
		g.finish(id, 0)
	}
	for i, r := range <-done {
		if r.Status() != fleet.LaneOK {
			t.Errorf("result[%d] = %+v, want ok: a window change never stops a lane", i, r)
		}
	}
}

func TestPool_AShrinkingWindowNeverStopsALaneInFlightAndOnlyDelaysTheNextAdmission(t *testing.T) {
	g, events := newGatedLaunch(), &eventLog{}
	w, _ := newWindow(4, 1, 4)
	done := runPool(Pool{Target: 4, Stage: StageEnforce, Window: w, Launch: g.launch, Publish: events.publish}, todosNamed("a", "b", "c", "d", "e", "f"))
	startN(t, g, 4)

	w.Conflict()
	w.Conflict()
	if w.Size() != 1 {
		t.Fatalf("window = %d, want 1 after two conflicts", w.Size())
	}
	for _, id := range []string{"a", "b", "c", "d"} {
		g.finish(id, 0)
	}
	if next := waitStart(t, g); next != "e" {
		t.Fatalf("next launch = %q, want e once the last of the 4 lanes in flight exited", next)
	}
	g.finish("e", 0)
	if next := waitStart(t, g); next != "f" {
		t.Fatalf("next launch = %q, want f", next)
	}
	finishAll(t, g, done, "f")
	if got := liveFields(events, CodeLaneBackfilled); !reflect.DeepEqual(got, []string{"1", "1"}) {
		t.Errorf("live lanes at each backfill = %v, want [1 1]: the first three exits freed slots outside the window of 1", got)
	}
}

func TestPool_AGrowingWindowFillsEveryFreedSlotAtTheNextExit(t *testing.T) {
	g, events := newGatedLaunch(), &eventLog{}
	w, _ := newWindow(2, 2, 4)
	done := runPool(Pool{Target: 4, Stage: StageEnforce, Window: w, Launch: g.launch, Publish: events.publish}, todosNamed("a", "b", "c", "d", "e", "f"))
	if first := startN(t, g, 2); !sameSet(first, []string{"a", "b"}) {
		t.Fatalf("first fill = %v, want a and b: the window opens at 2 of the target 4", first)
	}

	landings(w, 2)
	g.finish("a", 0)
	if refill := startN(t, g, 3); !sameSet(refill, []string{"c", "d", "e"}) {
		t.Fatalf("refill after the exit of a = %v, want c, d and e: the grown window of 4 takes 3 slots", refill)
	}
	g.finish("b", 0)
	if next := waitStart(t, g); next != "f" {
		t.Fatalf("next launch = %q, want f", next)
	}
	finishAll(t, g, done, "c", "d", "e", "f")
	if got := liveFields(events, CodeLaneBackfilled); !reflect.DeepEqual(got, []string{"2", "3", "4", "4"}) {
		t.Errorf("live lanes at each backfill = %v, want [2 3 4 4]", got)
	}
}

func TestPool_OnlyTheEnforceStageLetsTheWindowGateAdmission(t *testing.T) {
	for _, tc := range []struct {
		stage      string
		firstFill  []string
		backfilled []string
	}{
		{StageEnforce, []string{"1"}, []string{"1", "1", "1"}},
		{StageShadow, []string{"1", "2", "3", "4"}, nil},
		{"", []string{"1", "2", "3", "4"}, nil},
	} {
		t.Run("stage "+tc.stage, func(t *testing.T) {
			g, events := newGatedLaunch(), &eventLog{}
			w, _ := newWindow(1, 1, 4)
			done := runPool(Pool{Target: 4, Stage: tc.stage, Window: w, Launch: g.launch, Publish: events.publish}, todosNamed("a", "b", "c", "d"))
			for _, id := range []string{"a", "b", "c", "d"} {
				g.finish(id, 0)
			}
			for i, r := range <-done {
				if r.Status() != fleet.LaneOK {
					t.Errorf("result[%d] = %+v, want ok", i, r)
				}
			}
			if got := liveFields(events, CodeLaneLaunched); !reflect.DeepEqual(got, tc.firstFill) {
				t.Errorf("live lanes at each launch of the first fill = %v, want %v", got, tc.firstFill)
			}
			if got := liveFields(events, CodeLaneBackfilled); !reflect.DeepEqual(got, tc.backfilled) {
				t.Errorf("live lanes at each backfill = %v, want %v", got, tc.backfilled)
			}
		})
	}
}
