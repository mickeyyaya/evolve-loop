package fleet

import (
	"bytes"
	"strings"
	"testing"
)

func freshScopeIDs(specs []CycleSpec) []string {
	var ids []string
	for _, s := range specs {
		ids = append(ids, s.Scope...)
	}
	return ids
}

func containsID(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func TestWaveDispatch_SkipsConsumedTaskAndRefillsSlot(t *testing.T) {
	specs := []CycleSpec{
		{Scope: []string{"task-consumed"}},
		{Scope: []string{"task-live"}},
	}
	probe := func(id string) TaskFreshness {
		if id == "task-consumed" {
			return TaskFreshness{Fresh: false, Reason: "consumed: promoted processed cycle-748"}
		}
		return TaskFreshness{Fresh: true}
	}
	refillCalls := 0
	var gotExclude map[string]bool
	refill := func(exclude map[string]bool) (CycleSpec, bool) {
		refillCalls++
		gotExclude = exclude
		return CycleSpec{Scope: []string{"task-refill"}}, true
	}
	var warn bytes.Buffer

	kept, skipped := FreshenSpecs(specs, probe, refill, &warn)

	ids := freshScopeIDs(kept)
	if containsID(ids, "task-consumed") {
		t.Errorf("consumed task must not be dispatched: kept scope ids = %v", ids)
	}
	if !containsID(ids, "task-live") {
		t.Errorf("fresh task must survive the gate: kept scope ids = %v", ids)
	}
	if !containsID(ids, "task-refill") {
		t.Errorf("freed slot must be refilled from the backlog: kept scope ids = %v", ids)
	}
	if len(kept) != 2 {
		t.Errorf("wave width must be preserved by the refill: got %d specs, want 2", len(kept))
	}
	if refillCalls != 1 {
		t.Errorf("exactly one freed slot → exactly one refill call, got %d", refillCalls)
	}
	if gotExclude == nil || !gotExclude["task-consumed"] || !gotExclude["task-live"] {
		t.Errorf("refill exclude must contain every id the wave already owns (skipped AND kept): got %v", gotExclude)
	}
	if len(skipped) != 1 || skipped[0].TaskID != "task-consumed" {
		t.Fatalf("want exactly one skip record for task-consumed, got %+v", skipped)
	}
	if !strings.Contains(skipped[0].Reason, "consumed") {
		t.Errorf("skip record must carry the probe's reason, got %q", skipped[0].Reason)
	}
	log := warn.String()
	if !strings.Contains(log, "task-consumed") || !strings.Contains(log, "consumed") {
		t.Errorf("skip must be logged with id + reason, got log:\n%s", log)
	}
}

func TestWaveDispatch_SkipsDepsUnmetTaskWithReason(t *testing.T) {
	specs := []CycleSpec{
		{Scope: []string{"task-blocked"}},
		{Scope: []string{"task-live"}},
	}
	probe := func(id string) TaskFreshness {
		if id == "task-blocked" {
			return TaskFreshness{Fresh: false, Reason: "deps unmet: needs token-resolver-production-wiring"}
		}
		return TaskFreshness{Fresh: true}
	}
	refill := func(exclude map[string]bool) (CycleSpec, bool) {
		return CycleSpec{}, false
	}
	var warn bytes.Buffer

	kept, skipped := FreshenSpecs(specs, probe, refill, &warn)

	ids := freshScopeIDs(kept)
	if containsID(ids, "task-blocked") {
		t.Errorf("deps-unmet task must not be dispatched: kept scope ids = %v", ids)
	}
	if len(kept) != 1 || !containsID(ids, "task-live") {
		t.Errorf("empty backlog → slot stays empty, fresh lane survives: kept = %v", ids)
	}
	if len(skipped) != 1 || skipped[0].TaskID != "task-blocked" {
		t.Fatalf("want exactly one skip record for task-blocked, got %+v", skipped)
	}
	if !strings.Contains(skipped[0].Reason, "deps unmet") ||
		!strings.Contains(skipped[0].Reason, "token-resolver-production-wiring") {
		t.Errorf("skip reason must name the unmet dep, got %q", skipped[0].Reason)
	}
	log := warn.String()
	if !strings.Contains(log, "task-blocked") || !strings.Contains(log, "deps unmet") {
		t.Errorf("skip must be logged with id + reason, got log:\n%s", log)
	}
}

func TestBuildEmptyScope_AfterFreshnessGate_VerdictSkippedNotFail(t *testing.T) {
	cases := []struct {
		name             string
		gateRan          bool
		reportsNoInScope bool
		original         string
		want             string
	}{
		{"honest empty scope after gate is SKIPPED not FAIL", true, true, "FAIL", "SKIPPED"},
		{"honest empty scope after gate never counts as PASS", true, true, "PASS", "SKIPPED"},
		{"no gate ran: original FAIL preserved (pre-gate behavior)", false, true, "FAIL", "FAIL"},
		{"real in-scope FAIL is never masked", true, false, "FAIL", "FAIL"},
		{"real in-scope PASS untouched", true, false, "PASS", "PASS"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyEmptyScopeBuild(tc.gateRan, tc.reportsNoInScope, tc.original)
			if got != tc.want {
				t.Errorf("ClassifyEmptyScopeBuild(gateRan=%v, empty=%v, %q) = %q, want %q",
					tc.gateRan, tc.reportsNoInScope, tc.original, got, tc.want)
			}
		})
	}
}

func TestWaveDispatch_AllFresh_NoSkipNoRefill(t *testing.T) {
	specs := []CycleSpec{
		{Scope: []string{"task-a"}},
		{Scope: []string{"task-b", "task-c"}},
	}
	probe := func(id string) TaskFreshness { return TaskFreshness{Fresh: true} }
	refillCalls := 0
	refill := func(exclude map[string]bool) (CycleSpec, bool) {
		refillCalls++
		return CycleSpec{Scope: []string{"task-never"}}, true
	}
	var warn bytes.Buffer

	kept, skipped := FreshenSpecs(specs, probe, refill, &warn)

	if len(skipped) != 0 {
		t.Errorf("all-fresh wave must skip nothing, got %+v", skipped)
	}
	if refillCalls != 0 {
		t.Errorf("all-fresh wave must never call refill, got %d calls", refillCalls)
	}
	if warn.Len() != 0 {
		t.Errorf("all-fresh wave must log nothing, got:\n%s", warn.String())
	}
	if len(kept) != 2 ||
		strings.Join(kept[0].Scope, ",") != "task-a" ||
		strings.Join(kept[1].Scope, ",") != "task-b,task-c" {
		t.Errorf("all-fresh specs must pass through unchanged in order, got %+v", kept)
	}
}

func TestWaveDispatch_PartialStaleScope_FiltersIdKeepsSpec(t *testing.T) {
	specs := []CycleSpec{
		{Scope: []string{"task-consumed", "task-live"}},
	}
	probe := func(id string) TaskFreshness {
		if id == "task-consumed" {
			return TaskFreshness{Fresh: false, Reason: "consumed: promoted processed cycle-745"}
		}
		return TaskFreshness{Fresh: true}
	}
	refillCalls := 0
	refill := func(exclude map[string]bool) (CycleSpec, bool) {
		refillCalls++
		return CycleSpec{Scope: []string{"task-never"}}, true
	}
	var warn bytes.Buffer

	kept, skipped := FreshenSpecs(specs, probe, refill, &warn)

	if len(kept) != 1 || strings.Join(kept[0].Scope, ",") != "task-live" {
		t.Errorf("partially-stale spec must keep its slot with scope filtered to live ids, got %+v", kept)
	}
	if refillCalls != 0 {
		t.Errorf("a slot that still has live work must not be refilled, got %d refill calls", refillCalls)
	}
	if len(skipped) != 1 || skipped[0].TaskID != "task-consumed" {
		t.Errorf("the consumed id must be recorded as skipped, got %+v", skipped)
	}
}
