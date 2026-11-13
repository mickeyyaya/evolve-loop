package orchestrator

import (
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

func TestIdleCause_TheClosedSetHasSixCauses(t *testing.T) {
	want := []IdleCause{"family_held", "run_cap", "supply_empty", "admission_refused", "bench", "operator_stop"}
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
	want := "family_held, run_cap, supply_empty, admission_refused, bench or operator_stop"
	if idleCauseList != want {
		t.Errorf("idleCauseList = %q, want %q", idleCauseList, want)
	}
}
