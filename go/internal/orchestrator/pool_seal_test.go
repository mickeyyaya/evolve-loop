package orchestrator

import (
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func sealedEvents(l *eventLog) []signalcenter.Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []signalcenter.Event
	for _, e := range l.events {
		if e.Kind == signalcenter.KindCycleSealed {
			out = append(out, e)
		}
	}
	return out
}

func TestPool_AStoppedLaneSealsItsCycleWithTheLaneDeadlineTermination(t *testing.T) {
	g, events, clock := newGatedLaunch(), &eventLog{}, newFakeClock()
	located := map[string]int{"slow": 1955, "quick": 1956}
	pool := Pool{Target: 1, Launch: g.launch, Publish: events.publish,
		Locate: func(l Lane) (int, bool) { c, ok := located[l.Key]; return c, ok },
		Leases: NewLeases(leaseConfig(StageEnforce), clock, noHeartbeat, events.publish)}
	done := runPool(pool, []fleet.Todo{{ID: "slow"}, {ID: "quick"}})
	waitStart(t, g)
	clock.Advance(61 * time.Minute)
	if next := waitStart(t, g); next != "quick" {
		t.Fatalf("launch after the stop = %q, want quick", next)
	}
	g.finish("quick", 0)
	<-done

	seals := sealedEvents(events)
	if len(seals) != 1 {
		t.Fatalf("sealed events = %+v, want one seal for the stopped lane and none for the lane that exited by itself", seals)
	}
	got := seals[0]
	want := map[string]string{cyclestate.SealedTerminationField: cyclestate.CycleTerminationLaneDeadline, "todo": "slow"}
	if got.Cycle != 1955 || got.Module != signalcenter.ModuleLoop || got.Severity != signalcenter.SeverityInfo || got.Code != "" || !reflect.DeepEqual(got.Fields, want) {
		t.Errorf("seal = %+v, want cycle 1955 from the loop with fields %v", got, want)
	}
	if cyclestate.CycleTerminationLaneDeadline != "stopped: lane deadline" {
		t.Errorf("termination = %q, want the filed label", cyclestate.CycleTerminationLaneDeadline)
	}
}

func TestPool_AStoppedLaneWithNoLocatedCycleSaysSoLoudly(t *testing.T) {
	g, events, clock := newGatedLaunch(), &eventLog{}, newFakeClock()
	pool := Pool{Target: 1, Launch: g.launch, Publish: events.publish,
		Locate: func(Lane) (int, bool) { return 0, false },
		Leases: NewLeases(leaseConfig(StageEnforce), clock, noHeartbeat, events.publish)}
	done := runPool(pool, []fleet.Todo{{ID: "A"}})
	waitStart(t, g)
	clock.Advance(61 * time.Minute)
	<-done

	failed := events.withCode(CodeLaneRecoveryFailed)
	if len(sealedEvents(events)) != 0 || len(failed) != 1 || failed[0].Fields["step"] != "seal" || failed[0].Fields["todo"] != "A" {
		t.Errorf("seals = %+v failures = %+v, want no seal and one seal-step failure for A", sealedEvents(events), failed)
	}
}

func TestPool_AStoppedLaneWithNoPublisherIsStillLocatedAndReleased(t *testing.T) {
	g, claims, clock := newGatedLaunch(), &claimLog{}, newFakeClock()
	var located []string
	pool := Pool{Target: 1, Launch: g.launch, Claims: claims,
		Locate: func(l Lane) (int, bool) { located = append(located, l.Key); return 7, true },
		Leases: NewLeases(leaseConfig(StageEnforce), clock, noHeartbeat, nil)}
	done := runPool(pool, []fleet.Todo{{ID: "A"}})
	waitStart(t, g)
	clock.Advance(61 * time.Minute)
	<-done

	released, _, _ := claims.snapshot()
	if !reflect.DeepEqual(located, []string{"A"}) || !reflect.DeepEqual(released, []string{"A"}) {
		t.Errorf("located = %v released = %v, want A located and released with no publisher", located, released)
	}
}

func TestPool_TheDeadlineSealPassesTheSignalCenterAsACycleSeal(t *testing.T) {
	g, clock := newGatedLaunch(), newFakeClock()
	streams := t.TempDir()
	center := signalcenter.New()
	center.Subscribe(center.NDJSONSink(func(cycle int) string {
		return filepath.Join(streams, strconv.Itoa(cycle)+".ndjson")
	}))
	pool := Pool{Target: 1, Launch: g.launch, Publish: center.Emit,
		Locate: func(Lane) (int, bool) { return 1955, true },
		Leases: NewLeases(leaseConfig(StageEnforce), clock, noHeartbeat, center.Emit)}
	done := runPool(pool, []fleet.Todo{{ID: "A"}})
	waitStart(t, g)
	clock.Advance(61 * time.Minute)
	<-done
	center.Flush()

	chunk, err := signalcenter.ReadStream(filepath.Join(streams, "1955.ndjson"), 0)
	if err != nil || len(chunk.Events) != 1 {
		t.Fatalf("cycle stream = %+v, %v; want exactly the one seal", chunk.Events, err)
	}
	e := chunk.Events[0]
	if e.Kind != signalcenter.KindCycleSealed || e.Code != "" || e.Fields[cyclestate.SealedTerminationField] != cyclestate.CycleTerminationLaneDeadline {
		t.Errorf("written seal = %+v, want an INFO cycle.sealed with no drift code and the lane deadline termination", e)
	}
}

func TestOversight_TheLoopSealOfADeadlineStopIsNoVerdictAndKeepsTheMapping(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.addLane("A", 41)
	f.start()
	inFlight := []fleet.Todo{todoIn("A", "inboxrank")}
	f.send("cycle", phaseEvent(41, signalcenter.KindPhaseDispatched, epoch))
	stop := sealedEvent(41, "")
	stop.Module, stop.Fields = signalcenter.ModuleLoop, map[string]string{cyclestate.SealedTerminationField: cyclestate.CycleTerminationLaneDeadline}

	f.send("cycle", stop)
	if _, kept := f.over.cycles[41]; !kept || len(f.actions()) != 0 {
		t.Fatalf("mapping kept %t, actions %+v; want the deadline seal to record no verdict and keep the mapping", kept, f.actions())
	}
	f.send("cycle", sealedEvent(41, cyclestate.VerdictFAIL))

	if !f.admits(inFlight, todoIn("d", "inboxrank")) {
		t.Error("a FAIL seal after the deadline seal did not free the cells of A")
	}
}
