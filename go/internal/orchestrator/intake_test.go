package orchestrator

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func firstOf(_, pending []fleet.Todo) int {
	if len(pending) == 0 {
		return -1
	}
	return 0
}

func TestIntake_AFreedLaneHoldsNoCellsAndAHeldLaneHoldsThemAgain(t *testing.T) {
	graph := cellGraph(t, "inboxrank", "quotastate")
	in := NewIntake(4)
	admit := in.Gate(AdmitByCell(admission.Admitter{Graph: graph}))
	running := []fleet.Todo{todoIn("a", "inboxrank"), todoIn("b", "quotastate")}
	pending := []fleet.Todo{todoIn("d", "inboxrank")}

	if got := admit(running, pending); got != -1 {
		t.Fatalf("admit with a holding inboxrank = %d, want -1", got)
	}
	in.Free("a")
	if got := admit(running, pending); got != 0 {
		t.Errorf("admit after a's cells were freed = %d, want 0 (d)", got)
	}
	in.Hold("a")
	if got := admit(running, pending); got != -1 {
		t.Errorf("admit after a launched again = %d, want -1: a relaunch holds its cells", got)
	}
}

func TestIntake_TheLimitStopsNewAdmissionAndNeverTheLanesInFlight(t *testing.T) {
	in := NewIntake(3)
	admit := in.Gate(firstOf)
	two := []fleet.Todo{{ID: "a"}, {ID: "b"}}
	pending := []fleet.Todo{{ID: "c"}}

	if got := admit(two, pending); got != 0 {
		t.Fatalf("admit with 2 of 3 in flight = %d, want 0", got)
	}
	in.Resize(2)
	if got := admit(two, pending); got != -1 {
		t.Errorf("admit with 2 of 2 in flight = %d, want -1: a shrink stops the next admission only", got)
	}
	in.Resize(1)
	if got := admit(two, pending); got != -1 {
		t.Errorf("admit with 2 of 1 in flight = %d, want -1", got)
	}
	in.Resize(4)
	if got := admit(two, pending); got != 0 {
		t.Errorf("admit after the limit grew back = %d, want 0", got)
	}
}

func TestIntake_StopIsStickyAndEveryChangeThatCanAdmitKicksOnce(t *testing.T) {
	in := NewIntake(2)
	if in.Stopped() {
		t.Fatal("a new intake is stopped, want open")
	}
	in.Stop()
	if !in.Stopped() {
		t.Error("Stopped = false after Stop, want true")
	}
	select {
	case <-in.Wake():
		t.Fatal("Stop kicked the pool, want no kick: a stop admits nothing")
	default:
	}
	in.Free("a")
	in.Free("b")
	in.Resize(3)
	select {
	case <-in.Wake():
	default:
		t.Fatal("Free and Resize never kicked the pool")
	}
	select {
	case <-in.Wake():
		t.Fatal("three changes made more than one kick, want them coalesced")
	default:
	}
}

func TestPool_AKickAfterAFreeFillsTheOpenSlotBeforeAnyLaneExits(t *testing.T) {
	g, events := newGatedLaunch(), &eventLog{}
	in := NewIntake(3)
	admit, refused := refusals(AdmitByCell(admission.Admitter{Graph: cellGraph(t, "inboxrank", "quotastate")}))
	pool := Pool{Target: 3, Admit: admit, Launch: g.launch, Publish: events.publish, Intake: in}
	done := runPool(pool, []fleet.Todo{todoIn("a", "inboxrank"), todoIn("b", "quotastate"), todoIn("d", "inboxrank")})
	first := []string{waitStart(t, g), waitStart(t, g)}
	if !sameSet(first, []string{"a", "b"}) {
		t.Fatalf("first fill = %v, want a and b", first)
	}
	<-refused

	in.Free("a")
	if next := waitStart(t, g); next != "d" {
		t.Fatalf("after a's cells were freed the open slot took %q, want d while a still runs", next)
	}
	if got := len(events.withCode(CodeLaneBackfilled)); got != 1 {
		t.Errorf("backfilled events = %d, want 1 (d)", got)
	}
	for _, id := range []string{"a", "b", "d"} {
		g.finish(id, 0)
	}
	<-done
}

func TestPool_AStopOnTheIntakeEndsAdmissionAndLanesInFlightFinish(t *testing.T) {
	g := newGatedLaunch()
	in := NewIntake(2)
	pool := Pool{Target: 2, Concurrency: 1, Launch: g.launch, Intake: in, Admit: AdmitByCell(admission.Admitter{Graph: cellGraph(t, "inboxrank", "quotastate")})}
	done := runPool(pool, []fleet.Todo{todoIn("a", "inboxrank"), todoIn("b", "quotastate")})
	waitStart(t, g)

	in.Stop()
	g.finish("a", 0)
	results := <-done
	if results[0].Status() != fleet.LaneOK {
		t.Errorf("result[0] = %+v, want the lane in flight to finish ok", results[0])
	}
	if results[1].ExitCode != -1 {
		t.Errorf("result[1] = %+v, want b never launched after the stop", results[1])
	}
}

func TestPool_ALaunchClearsTheFreedMarkOfAnEarlierLifeOfTheSameTodo(t *testing.T) {
	g := newGatedLaunch()
	in := NewIntake(4)
	admit := AdmitByCell(admission.Admitter{Graph: cellGraph(t, "inboxrank")})
	in.Free("a")
	pool := Pool{Target: 2, Launch: g.launch, Intake: in, Admit: admit}
	done := runPool(pool, []fleet.Todo{todoIn("a", "inboxrank")})
	waitStart(t, g)

	got := in.Gate(admit)([]fleet.Todo{todoIn("a", "inboxrank")}, []fleet.Todo{todoIn("d", "inboxrank")})
	if got != -1 {
		t.Errorf("admit while the new life of a runs = %d, want -1: the launch must clear the freed mark", got)
	}
	g.finish("a", 0)
	<-done
}

func TestPool_AnIntakeWithNoAdmitFunctionRefusesEveryLaneLoudly(t *testing.T) {
	var launched atomic.Int64
	launch := func(context.Context, fleet.CycleSpec) (int, error) {
		launched.Add(1)
		return 0, nil
	}
	pool := Pool{Target: 2, Launch: launch, Intake: NewIntake(2)}

	results := pool.Run(context.Background(), []fleet.Todo{{ID: "a"}, {ID: "b"}})

	if len(results) != 2 {
		t.Fatalf("results = %d, want one for each todo", len(results))
	}
	for i, r := range results {
		if r.Index != i || r.ExitCode != -1 || !errors.Is(r.Err, errIntakeNeedsAdmit) {
			t.Errorf("result[%d] = %+v, want a refusal that names the missing admit function: the freed cells and the limit act through it", i, r)
		}
	}
	if n := launched.Load(); n != 0 {
		t.Errorf("launched %d lane(s), want no launch at all", n)
	}
}
