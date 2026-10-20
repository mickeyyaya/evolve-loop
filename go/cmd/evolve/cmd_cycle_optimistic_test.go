package main

import (
	"reflect"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func optimisticFailures(t *testing.T, env map[string]string, emit ...signalcenter.Event) []signalcenter.Event {
	t.Helper()
	var mu sync.Mutex
	var got []signalcenter.Event
	c := signalcenter.New()
	c.Subscribe(func(e signalcenter.Event) {
		mu.Lock()
		defer mu.Unlock()
		if e.Code == orchestrator.CodeOptimisticAdmissionFailed {
			got = append(got, e)
		}
	})
	watchOptimisticAdmission(c, func(k string) string { return env[k] })
	for _, e := range emit {
		c.Emit(e)
	}
	c.Flush()
	mu.Lock()
	defer mu.Unlock()
	return got
}

func orchestratorEvent(code signalcenter.Code, kind signalcenter.Kind, fields map[string]string) signalcenter.Event {
	return signalcenter.Event{Cycle: 7, Module: signalcenter.ModuleOrchestrator, Origin: "test", Kind: kind,
		Severity: signalcenter.SeverityWarn, Code: code, Reason: "r", Fields: fields}
}

func TestWatchOptimisticAdmission_AComposedGateFailureOfAnOptimisticLanePublishesTheTodoTheHolderAndTheGate(t *testing.T) {
	t.Parallel()
	env := map[string]string{ipcenv.FleetScopeKey: "router-todo", ipcenv.FleetReachHolderKey: "core-lane"}

	got := optimisticFailures(t, env,
		orchestratorEvent(core.CodeComposedGateDeclined, signalcenter.KindGateRejected, map[string]string{"gates": "unit,lint"}),
		orchestratorEvent(core.CodeFleetRebaseConflict, signalcenter.KindShipWarning, nil))

	want := map[string]string{"todo": "router-todo", "held_by": "core-lane", "gate": "unit,lint"}
	if len(got) != 1 || !reflect.DeepEqual(got[0].Fields, want) {
		t.Fatalf("failures = %+v, want one with %v", got, want)
	}
}

func TestWatchOptimisticAdmission_ARebaseConflictNamesTheRebaseGate(t *testing.T) {
	t.Parallel()
	env := map[string]string{ipcenv.FleetScopeKey: "router-todo", ipcenv.FleetReachHolderKey: "core-lane"}

	got := optimisticFailures(t, env, orchestratorEvent(core.CodeFleetRebaseConflict, signalcenter.KindShipWarning, map[string]string{"paths": "a.go"}))

	if len(got) != 1 || got[0].Fields["gate"] != "rebase" {
		t.Fatalf("failures = %+v, want one naming the rebase gate", got)
	}
}

func TestWatchOptimisticAdmission_ALaneWithoutAHolderPublishesNothing(t *testing.T) {
	t.Parallel()
	for name, env := range map[string]map[string]string{
		"no holder": {ipcenv.FleetScopeKey: "router-todo"},
		"no scope":  {ipcenv.FleetReachHolderKey: "core-lane"},
	} {
		if got := optimisticFailures(t, env, orchestratorEvent(core.CodeFleetRebaseConflict, signalcenter.KindShipWarning, nil)); len(got) != 0 {
			t.Errorf("%s: failures = %+v, want none", name, got)
		}
	}
}
