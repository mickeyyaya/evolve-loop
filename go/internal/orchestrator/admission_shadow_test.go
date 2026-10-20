package orchestrator

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func importGraph(t *testing.T) *admission.ModuleGraph {
	t.Helper()
	listing := admission.ModulePath + "/internal/core|" + admission.ModulePath + "/internal/router\n" +
		admission.ModulePath + "/internal/router|\n" +
		admission.ModulePath + "/cmd/evolve|" + admission.ModulePath + "/internal/core " + admission.ModulePath + "/internal/router\n"
	g, err := admission.NewModuleGraph(listing, 1000)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestAdmissionShadow_TheCodeIsRegisteredToTheLoopModule(t *testing.T) {
	m, ok := signalcenter.IsRegistered(CodeAdmissionShadow)
	if !ok || m != signalcenter.ModuleLoop || string(CodeAdmissionShadow) != "LOOP_ORCH_ADMISSION_SHADOW" {
		t.Errorf("IsRegistered(%q) = %q, %v, want the loop module", CodeAdmissionShadow, m, ok)
	}
}

func TestRecordShadow_PublishesTheTodoItsCellsAndItsHolder(t *testing.T) {
	events := &eventLog{}
	RecordShadow(events.publish)(admission.ShadowAdmission{ID: "router-todo", Rank: 3, Cells: []string{"internal/router", "internal/ipcenv"}, HeldBy: "core-lane"})
	got := events.withCode(CodeAdmissionShadow)
	if len(got) != 1 {
		t.Fatalf("events = %+v, want one admission shadow event", events.events)
	}
	want := map[string]string{"todo": "router-todo", "held_by": "core-lane", "rank": "3", "cells": "internal/router,internal/ipcenv"}
	if !reflect.DeepEqual(got[0].Fields, want) || got[0].Severity != signalcenter.SeverityInfo || got[0].Module != signalcenter.ModuleLoop {
		t.Errorf("event = %+v, want info severity in the loop module with fields %v", got[0], want)
	}
}

func TestRecordShadow_ANilPublisherDropsTheRecord(t *testing.T) {
	RecordShadow(nil)(admission.ShadowAdmission{ID: "x", HeldBy: "y"})
}

func TestAdmitByCell_AtShadowAHeldImportIsRecordedAndStillWaits(t *testing.T) {
	events := &eventLog{}
	admit := AdmitByCell(admission.Admitter{Graph: importGraph(t), OnShadow: RecordShadow(events.publish)})
	core := todoIn("core-todo", "core")
	router := todoIn("router-todo", "router")
	if got := admit([]fleet.Todo{core}, []fleet.Todo{router}); got != -1 {
		t.Fatalf("admit = %d, want -1: at shadow the reach rule still holds router behind core", got)
	}
	recorded := events.withCode(CodeAdmissionShadow)
	if len(recorded) != 1 || recorded[0].Fields["todo"] != "router-todo" || recorded[0].Fields["held_by"] != "core-todo" {
		t.Fatalf("recorded = %+v, want router-todo held by core-todo", recorded)
	}
}

func TestAdmitByCell_UnderTheOptimisticRuleACoreLaneDoesNotHoldARouterTodo(t *testing.T) {
	events := &eventLog{}
	admit := AdmitByCell(admission.Admitter{Graph: importGraph(t), Optimistic: true, OnShadow: RecordShadow(events.publish)})
	if got := admit([]fleet.Todo{todoIn("core-todo", "core")}, []fleet.Todo{todoIn("router-todo", "router")}); got != 0 {
		t.Fatalf("admit = %d, want 0: core only imports router", got)
	}
	if got := admit([]fleet.Todo{todoIn("core-todo", "core")}, []fleet.Todo{todoIn("core-two", "core")}); got != -1 {
		t.Errorf("admit of a second core todo = %d, want -1: two todos never edit one package at once", got)
	}
	if recorded := events.withCode(CodeAdmissionShadow); len(recorded) != 0 {
		t.Errorf("recorded = %+v, want none once the new rule governs", recorded)
	}
}
