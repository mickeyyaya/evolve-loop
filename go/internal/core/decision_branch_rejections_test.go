package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestPlanCycle_RecordsValidationRejections(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	led := &fakeLedger{}
	o := NewOrchestrator(&fakeStorage{}, led, buildRunners(nil))
	rej := []router.PlanRejection{
		{Phase: "frobnicate", Reason: "unknown-phase", Detail: "x"},
		{Phase: "ship", Reason: "ship-skips-audit", Detail: "y"},
	}
	o.recordPlanRejections(context.Background(), 7, CycleState{WorkspacePath: ws}, rej)

	raw, err := os.ReadFile(filepath.Join(ws, "advisor-rejections.json"))
	if err != nil {
		t.Fatalf("advisor-rejections.json not written: %v", err)
	}
	var got []router.PlanRejection
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("advisor-rejections.json is not valid json: %v", err)
	}
	if !reflect.DeepEqual(got, rej) {
		t.Errorf("rejections round-trip mismatch:\n got=%+v\nwant=%+v", got, rej)
	}

	var bound *LedgerEntry
	for i := range led.entries {
		if led.entries[i].Kind == "plan_rejections" {
			bound = &led.entries[i]
		}
	}
	if bound == nil {
		t.Fatalf("no plan_rejections ledger entry; entries=%+v", led.entries)
	}
	if bound.ArtifactSHA256 != sha256Hex(t, filepath.Join(ws, "advisor-rejections.json")) {
		t.Errorf("plan_rejections bound sha %q != sha256 of the written file", bound.ArtifactSHA256)
	}
}

func TestPlanCycle_PlanUnchangedWhenRejectionsRecorded(t *testing.T) {
	t.Parallel()
	plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: "build", Run: true}, {Phase: "ship", Run: true}}}

	wsA := t.TempDir()
	NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil)).
		recordPhasePlan(context.Background(), 1, CycleState{WorkspacePath: wsA}, plan, nil)
	base, _ := os.ReadFile(filepath.Join(wsA, "phase-plan.json"))

	wsB := t.TempDir()
	oB := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	before := append([]router.PhasePlanEntry(nil), plan.Entries...)
	oB.recordPlanRejections(context.Background(), 2, CycleState{WorkspacePath: wsB}, router.ValidatePlan(router.RouteInput{}, plan))
	oB.recordPhasePlan(context.Background(), 2, CycleState{WorkspacePath: wsB}, plan, nil)
	withRej, _ := os.ReadFile(filepath.Join(wsB, "phase-plan.json"))

	if string(base) != string(withRej) {
		t.Errorf("phase-plan.json changed when rejections recorded:\n%s\nvs\n%s", base, withRej)
	}
	if !reflect.DeepEqual(plan.Entries, before) {
		t.Error("ValidatePlan mutated the plan during recording")
	}
	if _, err := os.Stat(filepath.Join(wsB, "advisor-rejections.json")); err != nil {
		t.Errorf("advisor-rejections.json must be written alongside phase-plan.json: %v", err)
	}
}
