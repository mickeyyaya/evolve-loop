package orchestrator

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestPool_HoldWithNoRunningLaneStartsNothingAndDoesNotReturn(t *testing.T) {
	g, events, hold := newGatedLaunch(), &eventLog{}, fleet.NewFloorHold()
	hold.Set(true)
	done := runPool(Pool{Target: 2, Hold: hold, Launch: g.launch, Publish: events.publish}, todosNamed("a"))
	select {
	case id := <-g.started:
		t.Fatalf("launched %q under the hold", id)
	case <-done:
		t.Fatal("the pool returned while held with no lane")
	case <-time.After(150 * time.Millisecond):
	}
	hold.Set(false)
	if id := waitStart(t, g); id != "a" {
		t.Fatalf("launched %q after the clear, want a", id)
	}
	finishAll(t, g, done, "a")
}

func TestPool_HoldLetsTwoRunningLanesFinish(t *testing.T) {
	g, events, hold := newGatedLaunch(), &eventLog{}, fleet.NewFloorHold()
	done := runPool(Pool{Target: 2, Hold: hold, Launch: g.launch, Publish: events.publish}, todosNamed("a", "b", "c"))
	startN(t, g, 2)
	hold.Set(true)
	g.finish("a", 0)
	g.finish("b", 0)
	select {
	case id := <-g.started:
		t.Fatalf("a third lane %q started under the hold", id)
	case <-time.After(150 * time.Millisecond):
	}
	hold.Set(false)
	if id := waitStart(t, g); id != "c" {
		t.Fatalf("launched %q after the clear, want c", id)
	}
	finishAll(t, g, done, "c")
}

func TestPool_AHoldReportsDiskFloorNotUnderfill(t *testing.T) {
	g, events, hold := newGatedLaunch(), &eventLog{}, fleet.NewFloorHold()
	idled := make(chan struct{}, 4)
	publish := func(e signalcenter.Event) {
		events.publish(e)
		if e.Code == CodePoolIdle {
			idled <- struct{}{}
		}
	}
	pool := Pool{Target: 2, MinLanes: 2, Hold: hold, Launch: g.launch, Publish: publish,
		Resupply: func([]fleet.Todo) []fleet.Todo { return nil }}
	done := runPool(pool, todosNamed("a"))
	startN(t, g, 1)
	hold.Set(true)
	g.finish("a", 0)
	<-idled
	hold.Set(false)
	<-done
	if got := fieldsOf(events, CodePoolIdle, "cause"); len(got) == 0 || got[0] != string(CauseDiskFloor) {
		t.Fatalf("idle causes = %v, want disk_floor first", got)
	}
	if n := len(events.withCode(CodePoolUnderfilled)); n != 0 {
		t.Fatalf("a hold raised %d underfill WARNs", n)
	}
	holds := events.withCode(CodePoolFloorHold)
	if len(holds) == 0 || holds[0].Fields["held"] != "true" || holds[0].Fields["live"] == "" {
		t.Fatalf("floor hold events = %+v, want the first at the start of the hold with the live count", holds)
	}
}

func TestPool_TheFloorHoldTransitionHasARegisteredLoopOrchCode(t *testing.T) {
	t.Parallel()
	if transitionCodes[fleet.PoolFloorHold] != CodePoolFloorHold || CodePoolFloorHold != "LOOP_ORCH_POOL_FLOOR_HOLD" {
		t.Fatalf("PoolFloorHold maps to %q", transitionCodes[fleet.PoolFloorHold])
	}
	if owner, ok := signalcenter.IsRegistered(CodePoolFloorHold); !ok || owner != signalcenter.ModuleLoop {
		t.Fatalf("registered %v owner %q, want the loop module", ok, owner)
	}
}

func TestIdleCause_AHeldTransitionNamesDiskFloorBeforeEveryOtherCause(t *testing.T) {
	t.Parallel()
	tr := fleet.PoolTransition{Kind: fleet.PoolIdle, Pending: 0, Stopped: true, Capped: true, Held: true}
	if got := (Pool{}).idleCause(tr); got != CauseDiskFloor {
		t.Fatalf("idleCause = %q, want %q", got, CauseDiskFloor)
	}
	if !CauseDiskFloor.Valid() {
		t.Fatal("disk_floor is not in the closed set of idle causes")
	}
}
