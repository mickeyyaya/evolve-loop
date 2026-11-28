package orchestrator

import (
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestPool_UnderfilledEventNamesTheRunCapCause(t *testing.T) {
	g, events := newGatedLaunch(), &eventLog{}
	done := runPool(Pool{Target: 4, MinLanes: 4, MaxLanes: 2, Launch: g.launch, Publish: events.publish}, todosNamed("a", "b", "c"))
	startN(t, g, 2)

	g.finish("a", 0)
	g.finish("b", 0)
	<-done

	for _, code := range []signalcenter.Code{CodePoolUnderfilled, CodePoolIdle} {
		got := fieldsOf(events, code, "cause")
		if len(got) == 0 {
			t.Fatalf("%s events = 0, want the idle transitions of a capped run", code)
		}
		for _, c := range got {
			if c != string(CauseRunCap) {
				t.Errorf("%s cause = %q, want %q: the run launched run_max_lanes lanes", code, c, CauseRunCap)
			}
		}
	}
}

func TestIdleCause_EachTransitionNamesOneCauseOfTheClosedSet(t *testing.T) {
	held := NewFamilyHolds()
	held.Record(admission.Decision{ID: "ctx-b", Cells: []string{admission.FamilyCellPrefix + "ctx"}, BlockedBy: "ctx-a", FamilyHold: true})
	if held.Summary() == "" {
		t.Fatal("fixture: the family hold was not recorded")
	}
	yes, no := func() bool { return true }, func() bool { return false }
	idle := fleet.PoolTransition{Kind: fleet.PoolIdle, Live: 1, Target: 4, Pending: 3}
	stopped := fleet.PoolTransition{Kind: fleet.PoolIdle, Live: 1, Target: 4, Pending: 3, Stopped: true, Capped: true}
	capped := fleet.PoolTransition{Kind: fleet.PoolIdle, Live: 1, Target: 4, Pending: 3, Capped: true}
	empty := fleet.PoolTransition{Kind: fleet.PoolIdle, Pending: 0}
	tests := []struct {
		name string
		pool Pool
		tr   fleet.PoolTransition
		want IdleCause
	}{
		{"operator stop beats the run cap", Pool{}, stopped, CauseOperatorStop},
		{"run cap", Pool{}, capped, CauseRunCap},
		{"run cap beats family held", Pool{FamilyHolds: held}, capped, CauseRunCap},
		{"bench", Pool{Benched: yes}, idle, CauseBench},
		{"no bench", Pool{Benched: no}, idle, CauseAdmissionRefused},
		{"family held beats bench", Pool{Benched: yes, FamilyHolds: held}, idle, CauseFamilyHeld},
		{"family held beats supply empty", Pool{FamilyHolds: held}, empty, CauseFamilyHeld},
		{"supply empty beats bench", Pool{Benched: yes}, empty, CauseSupplyEmpty},
		{"family held", Pool{FamilyHolds: held}, idle, CauseFamilyHeld},
		{"supply empty", Pool{}, empty, CauseSupplyEmpty},
		{"admission refused", Pool{}, idle, CauseAdmissionRefused},
		{"held transition", Pool{}, fleet.PoolTransition{Kind: fleet.PoolHeld, Pending: 2}, CauseAdmissionRefused},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.pool.idleCause(tc.tr)
			if got != tc.want {
				t.Errorf("idleCause = %q, want %q", got, tc.want)
			}
			if !got.Valid() {
				t.Errorf("cause %q is outside the closed set", got)
			}
		})
	}
}

func TestIdleCause_TheClosedSetHasNineCauses(t *testing.T) {
	want := []IdleCause{"family_held", "run_cap", "supply_empty", "admission_refused", "bench", "operator_stop", "host_pressure", "system_halt", "disk_floor"}
	if len(IdleCauses) != len(want) {
		t.Fatalf("IdleCauses = %v, want %v", IdleCauses, want)
	}
	for i, c := range want {
		if IdleCauses[i] != c || !c.Valid() {
			t.Errorf("IdleCauses[%d] = %q, want %q", i, IdleCauses[i], c)
		}
	}
	for _, bad := range []IdleCause{"", "unknown", "RUN_CAP"} {
		if bad.Valid() {
			t.Errorf("%q is valid, want a refusal", bad)
		}
	}
}

func TestPublish_EmitIdleStampsTheCauseAndKeepsTheCallerFields(t *testing.T) {
	events := &eventLog{}
	fields := map[string]string{"live": "1"}
	Publish(events.publish).emitIdle(CodePoolIdle, "r", fields, CauseBench)
	got := events.withCode(CodePoolIdle)
	if len(got) != 1 || got[0].Fields["cause"] != "bench" || got[0].Fields["live"] != "1" || got[0].Origin != "Pool.Run" {
		t.Errorf("events = %+v, want one idle event with cause=bench and live=1", got)
	}
	if _, ok := fields["cause"]; ok {
		t.Error("emitIdle wrote the cause into the caller's map")
	}
	Publish(nil).emitIdle(CodePoolIdle, "r", nil, CauseBench)
}

func TestPublish_EmitIdleDerivesTheSeverityFromTheCode(t *testing.T) {
	events := &eventLog{}
	Publish(events.publish).emitIdle(CodePoolIdle, "r", nil, CauseBench)
	Publish(events.publish).emitIdle(CodePoolUnderfilled, "r", nil, CauseBench)
	if got := events.withCode(CodePoolIdle); len(got) != 1 || got[0].Severity != signalcenter.SeverityInfo {
		t.Errorf("idle events = %+v, want one INFO", got)
	}
	if got := events.withCode(CodePoolUnderfilled); len(got) != 1 || got[0].Severity != signalcenter.SeverityWarn {
		t.Errorf("underfill events = %+v, want one WARN", got)
	}
}

func TestPool_TransitionPublishesTheCauseOfAnIdleTransition(t *testing.T) {
	events := &eventLog{}
	p := Pool{Publish: events.publish, MinLanes: 2}
	p.transition(fleet.PoolTransition{Kind: fleet.PoolIdle, Live: 0, Target: 2, Pending: 0})
	p.transition(fleet.PoolTransition{Kind: fleet.PoolHeld, Live: 0, Target: 2, Pending: 1})
	for code, want := range map[signalcenter.Code]IdleCause{CodePoolIdle: CauseSupplyEmpty} {
		got := events.withCode(code)
		if len(got) != 1 || got[0].Fields["cause"] != string(want) {
			t.Errorf("%s events = %+v, want one with cause %s", code, got, want)
		}
	}
	under := events.withCode(CodePoolUnderfilled)
	if len(under) != 2 || under[0].Fields["cause"] != string(CauseSupplyEmpty) || under[1].Fields["cause"] != string(CauseAdmissionRefused) {
		t.Errorf("underfill events = %+v, want causes supply_empty then admission_refused", under)
	}
}

func TestIdleCauseList_NamesEveryCauseOfTheClosedSet(t *testing.T) {
	want := "family_held, run_cap, supply_empty, admission_refused, bench, operator_stop, host_pressure, system_halt or disk_floor"
	if idleCauseList != want {
		t.Errorf("idleCauseList = %q, want %q", idleCauseList, want)
	}
}

func TestPool_ABrakedPoolPastItsRunCapNamesTheOperatorStop(t *testing.T) {
	g, events := newGatedLaunch(), &eventLog{}
	var braked atomic.Bool
	done := runPool(Pool{Target: 4, MinLanes: 4, MaxLanes: 2, Launch: g.launch, Publish: events.publish, Halted: braked.Load}, todosNamed("a", "b", "c"))
	startN(t, g, 2)

	braked.Store(true)
	g.finish("a", 0)
	g.finish("b", 0)
	<-done

	got := fieldsOf(events, CodePoolIdle, "cause")
	if len(got) == 0 {
		t.Fatalf("%s events = 0, want the idle transitions of a braked run", CodePoolIdle)
	}
	for _, c := range got {
		if c != string(CauseOperatorStop) {
			t.Errorf("%s cause = %q, want %q: the brake drained a pool that was also at its cap", CodePoolIdle, c, CauseOperatorStop)
		}
	}
	if n := len(events.withCode(CodePoolUnderfilled)); n != 0 {
		t.Errorf("%s events = %d, want 0: an operator stop is not an underfill", CodePoolUnderfilled, n)
	}
}

func TestIdleCause_SystemHaltIsNotOperatorStop(t *testing.T) {
	halted := &atomic.Bool{}
	halted.Store(true)
	pool := Pool{sysHalt: halted}
	stopped := fleet.PoolTransition{Kind: fleet.PoolIdle, Pending: 3, Stopped: true}
	if got := pool.idleCause(stopped); got != CauseSystemHalt || !got.Valid() {
		t.Errorf("idleCause = %q, want %q: a system-failure halt is not an operator stop", got, CauseSystemHalt)
	}
	if got := (Pool{}).idleCause(stopped); got != CauseOperatorStop {
		t.Errorf("idleCause without a halt = %q, want %q", got, CauseOperatorStop)
	}
}

func TestIdleCause_AnAdmissionRefusalBeatsAnUnrelatedBench(t *testing.T) {
	yes, no := func() bool { return true }, func() bool { return false }
	refuse := func() *FamilyHolds {
		h := NewFamilyHolds()
		h.Record(admission.Decision{ID: "todo-a", Rank: 0, Cells: []string{"internal/x", "internal/y"}, BlockedBy: "lane-1"})
		return h
	}
	familyHeld := func() *FamilyHolds {
		h := NewFamilyHolds()
		h.Record(admission.Decision{ID: "todo-f", Cells: []string{admission.FamilyCellPrefix + "x"}, FamilyHold: true, BlockedBy: "lane-2"})
		return h
	}
	idle := fleet.PoolTransition{Kind: fleet.PoolIdle, Live: 1, Target: 4, Pending: 3}
	tests := []struct {
		name     string
		pool     Pool
		want     IdleCause
		wantAlso string
	}{
		{"refused and benched", Pool{Benched: yes, FamilyHolds: refuse()}, CauseAdmissionRefused, "bench"},
		{"refused and not benched", Pool{Benched: no, FamilyHolds: refuse()}, CauseAdmissionRefused, ""},
		{"refused without a bench probe", Pool{FamilyHolds: refuse()}, CauseAdmissionRefused, ""},
		{"family hold and benched", Pool{Benched: yes, FamilyHolds: familyHeld()}, CauseFamilyHeld, ""},
		{"benched and nothing refused", Pool{Benched: yes, FamilyHolds: NewFamilyHolds()}, CauseBench, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.pool.idleCause(idle); got != tc.want {
				t.Errorf("idleCause = %q, want %q", got, tc.want)
			}
			if got := tc.pool.idleFields(tc.want)["also"]; got != tc.wantAlso {
				t.Errorf("also = %q, want %q", got, tc.wantAlso)
			}
		})
	}
}

func TestPool_IdleSignalsNameTheRefusedTodoAndItsBlockingCell(t *testing.T) {
	events := &eventLog{}
	holds := NewFamilyHolds()
	for i, id := range []string{"t6", "t5", "t4", "t3", "t2", "t1"} {
		holds.Record(admission.Decision{ID: id, Rank: 6 - i, Cells: []string{"internal/cell-" + id}, BlockedBy: "lane-9"})
	}
	p := Pool{MinLanes: 4, Publish: events.publish, FamilyHolds: holds, Benched: func() bool { return true }}
	p.transition(fleet.PoolTransition{Kind: fleet.PoolIdle, Live: 1, Target: 4, Pending: 6})
	for _, code := range []signalcenter.Code{CodePoolIdle, CodePoolUnderfilled} {
		got := events.withCode(code)
		if len(got) != 1 {
			t.Fatalf("%s events = %d, want 1", code, len(got))
		}
		f := got[0].Fields
		if f["cause"] != "admission_refused" || f["also"] != "bench" {
			t.Errorf("%s cause/also = %q/%q, want admission_refused/bench", code, f["cause"], f["also"])
		}
		if f["refused"] != "t1,t2,t3,t4,t5" {
			t.Errorf("%s refused = %q, want the five best-ranked todos", code, f["refused"])
		}
		if !strings.HasPrefix(f["refused_by"], "t1 (footprint internal/cell-t1, held by lane-9); t2") {
			t.Errorf("%s refused_by = %q, want the blocking cell and lane of each todo", code, f["refused_by"])
		}
	}
}

func TestFamilyHolds_RefusedIsNilSafeAndForgetsALaunchedTodo(t *testing.T) {
	var nilHolds *FamilyHolds
	if nilHolds.Refused(3) != nil || nilHolds.refusedCount() != 0 {
		t.Error("a nil FamilyHolds must report no refusal")
	}
	h := NewFamilyHolds()
	h.Record(admission.Decision{ID: "b", Rank: 1, BlockedBy: "l"})
	h.Record(admission.Decision{ID: "a", Rank: 1, BlockedBy: "l"})
	if got := h.Refused(9); len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Errorf("Refused = %+v, want a then b by rank then id", got)
	}
	h.forget("a")
	if h.refusedCount() != 1 {
		t.Errorf("refusedCount = %d, want 1 after the launch of a", h.refusedCount())
	}
}

func TestPool_HeldSignalNamesTheRefusedTodoAndItsBlockingCell(t *testing.T) {
	events := &eventLog{}
	holds := NewFamilyHolds()
	holds.Record(admission.Decision{ID: "t1", Rank: 1, Cells: []string{"internal/cell-t1"}, BlockedBy: "lane-9"})
	p := Pool{Publish: events.publish, FamilyHolds: holds, Benched: func() bool { return true }}
	p.transition(fleet.PoolTransition{Kind: fleet.PoolHeld, Live: 1, Target: 4, Pending: 1})
	got := events.withCode(CodePoolHeld)
	if len(got) != 1 {
		t.Fatalf("%s events = %d, want 1", CodePoolHeld, len(got))
	}
	f := got[0].Fields
	if f["refused"] != "t1" || f["refused_by"] != "t1 (footprint internal/cell-t1, held by lane-9)" || f["also"] != "bench" {
		t.Errorf("held fields = %v, want refused t1, its footprint and lane, and also=bench", f)
	}
}

func TestFamilyHolds_RefusedOrdersByRankBeforeId(t *testing.T) {
	h := NewFamilyHolds()
	h.Record(admission.Decision{ID: "a", Rank: 2})
	h.Record(admission.Decision{ID: "b", Rank: 1})
	got := h.Refused(9)
	if len(got) != 2 || got[0].ID != "b" || got[1].ID != "a" {
		t.Errorf("Refused = %+v, want b then a: the rank order is the inverse of the id order", got)
	}
}

func TestFamilyHolds_SummaryDropsATodoThatIsNoLongerFamilyHeld(t *testing.T) {
	h := NewFamilyHolds()
	family := admission.Decision{ID: "a", Cells: []string{admission.FamilyCellPrefix + "x"}, FamilyHold: true, BlockedBy: "l"}
	h.Record(family)
	if h.Summary() == "" {
		t.Fatal("Summary is empty, want the family-held todo")
	}
	family.FamilyHold = false
	h.Record(family)
	if got := h.Summary(); got != "" {
		t.Errorf("Summary = %q, want empty once the todo is refused for another reason", got)
	}
}

func TestPool_AFreshenDropForgetsTheRefusalOfTheTodo(t *testing.T) {
	holds := NewFamilyHolds()
	holds.Record(admission.Decision{ID: "t1", Rank: 1, BlockedBy: "lane-9"})
	p := Pool{FamilyHolds: holds, Freshen: func(fleet.CycleSpec) (fleet.CycleSpec, bool) { return fleet.CycleSpec{}, false }}
	if _, ok := p.prepare(fleet.Todo{ID: "t1"}, fleet.CycleSpec{}); ok {
		t.Fatal("prepare ok = true, want the drop")
	}
	if got := holds.Refused(9); len(got) != 0 {
		t.Errorf("Refused = %+v, want none: the dropped todo is not refused any more", got)
	}
}
