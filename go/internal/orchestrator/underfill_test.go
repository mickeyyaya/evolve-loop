package orchestrator

import (
	"reflect"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func fieldsOf(events *eventLog, code signalcenter.Code, key string) []string {
	var out []string
	for _, e := range events.withCode(code) {
		out = append(out, e.Fields[key])
	}
	return out
}

func TestUnderfill_TheCodeIsRegisteredToTheLoopModule(t *testing.T) {
	m, ok := signalcenter.IsRegistered(CodePoolUnderfilled)
	if !ok || m != signalcenter.ModuleLoop || string(CodePoolUnderfilled) != "LOOP_ORCH_POOL_UNDERFILLED" {
		t.Errorf("IsRegistered(%q) = %q, %v, want the loop module", CodePoolUnderfilled, m, ok)
	}
}

func TestUnderfill_AnIdleTransitionBelowMinLanesWarnsOnceWithTheFacts(t *testing.T) {
	tests := []struct {
		name      string
		minLanes  int
		halted    func() bool
		tr        fleet.PoolTransition
		wantWarns int
	}{
		{"idle at 1 of 4 below min 4", 4, nil, fleet.PoolTransition{Kind: fleet.PoolIdle, Live: 1, Target: 4, Pending: 2}, 1},
		{"idle at 3 of 4 below min 4", 4, nil, fleet.PoolTransition{Kind: fleet.PoolIdle, Live: 3, Target: 4}, 1},
		{"idle at 0 with an empty supply", 4, nil, fleet.PoolTransition{Kind: fleet.PoolIdle, Live: 0, Target: 4}, 1},
		{"idle at min", 4, nil, fleet.PoolTransition{Kind: fleet.PoolIdle, Live: 4, Target: 4}, 0},
		{"idle above min", 2, nil, fleet.PoolTransition{Kind: fleet.PoolIdle, Live: 3, Target: 4}, 0},
		{"min lanes unset", 0, nil, fleet.PoolTransition{Kind: fleet.PoolIdle, Live: 1, Target: 4}, 0},
		{"operator brake engaged", 4, func() bool { return true }, fleet.PoolTransition{Kind: fleet.PoolIdle, Live: 1, Target: 4}, 0},
		{"brake released", 4, func() bool { return false }, fleet.PoolTransition{Kind: fleet.PoolIdle, Live: 1, Target: 4}, 1},
		{"launched below min", 4, nil, fleet.PoolTransition{Kind: fleet.PoolLaunched, Todo: "a", Live: 1, Target: 4}, 0},
		{"exited below min", 4, nil, fleet.PoolTransition{Kind: fleet.PoolExited, Todo: "a", Live: 0, Target: 4}, 0},
		{"backfilled below min", 4, nil, fleet.PoolTransition{Kind: fleet.PoolBackfilled, Todo: "b", Live: 1, Target: 4}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			events := &eventLog{}
			Pool{MinLanes: tc.minLanes, Halted: tc.halted, Publish: events.publish}.transition(tc.tr)

			warns := events.withCode(CodePoolUnderfilled)
			if len(warns) != tc.wantWarns {
				t.Fatalf("underfilled events = %d, want %d", len(warns), tc.wantWarns)
			}
			if tc.wantWarns == 0 {
				return
			}
			w := warns[0]
			want := map[string]string{"live": strconv.Itoa(tc.tr.Live), "target": strconv.Itoa(tc.tr.Target), "min_lanes": strconv.Itoa(tc.minLanes), "ready": strconv.Itoa(tc.tr.Pending)}
			if w.Severity != signalcenter.SeverityWarn || w.Kind != signalcenter.KindLoopWave || w.Module != signalcenter.ModuleLoop || !reflect.DeepEqual(w.Fields, want) {
				t.Errorf("event = %+v, want a loop.wave WARN with fields %v", w, want)
			}
			if got := len(events.withCode(CodePoolIdle)); got != 1 {
				t.Errorf("idle events = %d, want the INFO idle event kept beside the WARN", got)
			}
		})
	}
}

func TestUnderfill_ARunningPoolWarnsOnEveryIdleTransitionBelowMinLanes(t *testing.T) {
	g, events := newGatedLaunch(), &eventLog{}
	sharedFile := []string{"go/internal/x/f.go"}
	backlog := []fleet.Todo{{ID: "a", Files: sharedFile}, {ID: "b", Files: sharedFile}, {ID: "c", Files: sharedFile}}
	done := runPool(Pool{Target: 4, MinLanes: 4, Launch: g.launch, Publish: events.publish}, backlog)

	for _, id := range []string{"a", "b", "c"} {
		if got := waitStart(t, g); got != id {
			t.Fatalf("launched %q, want %q: one cell serves one lane at a time", got, id)
		}
		g.finish(id, 0)
	}
	<-done

	if got, want := fieldsOf(events, CodePoolUnderfilled, "live"), []string{"1", "1", "0"}; !reflect.DeepEqual(got, want) {
		t.Errorf("underfilled live = %v, want %v: one WARN for each idle transition", got, want)
	}
	if got, want := fieldsOf(events, CodePoolUnderfilled, "ready"), []string{"1", "0", "0"}; !reflect.DeepEqual(got, want) {
		t.Errorf("underfilled ready = %v, want %v: the todos still pending at each idle", got, want)
	}
	if got, want := len(events.withCode(CodePoolIdle)), len(events.withCode(CodePoolUnderfilled)); got != want {
		t.Errorf("idle events = %d, underfilled = %d, want one WARN for each idle", got, want)
	}
}

func TestUnderfill_NoWarnWhileTheLiveCountHoldsMinLanes(t *testing.T) {
	g, events := newGatedLaunch(), &eventLog{}
	done := runPool(Pool{Target: 2, MinLanes: 1, Launch: g.launch, Publish: events.publish}, todosNamed("a", "b"))
	startN(t, g, 2)

	g.finish("a", 0)
	g.finish("b", 0)
	<-done

	if got, want := fieldsOf(events, CodePoolUnderfilled, "live"), []string{"0"}; !reflect.DeepEqual(got, want) {
		t.Errorf("underfilled live = %v, want only the exit that drops under min_lanes 1 (%v): the first idle at live 1 holds it", got, want)
	}
	if got := len(events.withCode(CodePoolIdle)); got != 2 {
		t.Errorf("idle events = %d, want 2", got)
	}
}

func TestUnderfill_NoWarnWhileTheOperatorBrakeIsEngaged(t *testing.T) {
	g, events := newGatedLaunch(), &eventLog{}
	var brake atomic.Bool
	done := runPool(Pool{Target: 4, MinLanes: 4, Launch: g.launch, Publish: events.publish, Halted: brake.Load}, todosNamed("a", "b"))
	startN(t, g, 2)

	brake.Store(true)
	g.finish("a", 0)
	g.finish("b", 0)
	<-done

	if got := len(events.withCode(CodePoolUnderfilled)); got != 0 {
		t.Errorf("underfilled events = %d, want 0: a stopped pool is not starved", got)
	}
	if got := len(events.withCode(CodePoolIdle)); got == 0 {
		t.Errorf("idle events = 0, want the INFO idle events to stay")
	}
}
