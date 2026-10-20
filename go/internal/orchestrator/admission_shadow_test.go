package orchestrator

import (
	"io"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
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
	RecordShadow(events.publish)(admission.ShadowAdmission{ID: "router-todo", Rank: 3, Cells: []string{"internal/router", "internal/ipcenv"}, HeldBy: "core-lane", Reason: "reach meets internal/router"})
	got := events.withCode(CodeAdmissionShadow)
	if len(got) != 1 {
		t.Fatalf("events = %+v, want one admission shadow event", events.events)
	}
	want := map[string]string{"todo": "router-todo", "held_by": "core-lane", "rank": "3", "cells": "internal/router,internal/ipcenv", "reason": "reach meets internal/router"}
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

func TestRecordShadow_TwoPassesOverOneHeldTodoPublishOnce(t *testing.T) {
	events := &eventLog{}
	admit := AdmitByCell(admission.Admitter{Graph: importGraph(t), OnShadow: RecordShadow(events.publish)})
	inFlight := []fleet.Todo{todoIn("core-todo", "core")}
	pending := []fleet.Todo{todoIn("router-todo", "router")}
	for range 2 {
		if got := admit(inFlight, pending); got != -1 {
			t.Fatalf("admit = %d, want -1: at shadow the reach rule still holds router behind core", got)
		}
	}
	got := events.withCode(CodeAdmissionShadow)
	if len(got) != 1 {
		t.Fatalf("shadow events = %+v, want one: the pool pass repeats, the held todo and its holder do not change", got)
	}
	if got[0].Fields["reason"] != "reach meets internal/router" {
		t.Errorf("reason = %q, want the package where the edit of router-todo meets the reach of core-todo", got[0].Fields["reason"])
	}
}

func TestRecordShadow_TheTodoAndItsHolderAreTheKeyNotTheReason(t *testing.T) {
	events := &eventLog{}
	record := RecordShadow(events.publish)
	record(admission.ShadowAdmission{ID: "r", HeldBy: "lane-core", Reason: "reach meets internal/router"})
	record(admission.ShadowAdmission{ID: "r", HeldBy: "lane-core", Reason: "reach meets internal/clihealth"})
	record(admission.ShadowAdmission{ID: "r", HeldBy: "lane-other", Reason: "reach meets internal/router"})
	record(admission.ShadowAdmission{ID: "s", HeldBy: "lane-core", Reason: "reach meets internal/router"})
	if got := events.withCode(CodeAdmissionShadow); len(got) != 3 {
		t.Fatalf("shadow events = %+v, want 3: one for each todo and holder", got)
	}
}

func TestCensus_AtEnforceACandidateThatOnlyMeetsARunningLaneReachIsAdmissible(t *testing.T) {
	for optimistic, want := range map[bool]map[string]int{false: {ClassAdmissible: 0, ClassWaitingCell: 1}, true: {ClassAdmissible: 1, ClassWaitingCell: 0}} {
		root := t.TempDir()
		inbox := filepath.Join(root, ".evolve", "inbox")
		writeItem(t, inbox, "r", map[string]any{"files": fileIn("router")})
		c := Census{
			Inbox:      inboxmover.Options{InboxDir: inbox, Stderr: io.Discard},
			Rank:       inboxrank.Inputs{Config: policy.Policy{}.InboxPriorityConfig(), Now: epoch},
			Protected:  guardsProtected,
			Graph:      importGraph(t),
			InFlight:   []admission.Holding{{Lane: "lane-core", Todo: todoIn("c", "core")}},
			Optimistic: optimistic,
			Now:        func() time.Time { return epoch },
		}
		got, err := c.Count()
		if err != nil {
			t.Fatal(err)
		}
		if got.Counts[ClassAdmissible] != want[ClassAdmissible] || got.Counts[ClassWaitingCell] != want[ClassWaitingCell] {
			t.Errorf("optimistic %v: counts = %v, want %v: lane-core only imports the router that r edits", optimistic, got.Counts, want)
		}
	}
}
