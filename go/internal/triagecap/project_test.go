package triagecap

import (
	"encoding/json"
	"os"
	"testing"
)

// realReport is a production-shaped report: canonical headings, "- {id}: {action} — metadata" items, reason= tails.
const realReport = `<!-- challenge-token: abc -->
<!-- ANCHOR:triage_decision -->
# Triage Decision — Cycle 322

cycle_size_estimate: small
phase_skip: []

## top_n (commit to THIS cycle)
- modelcatalog-write-error-paths: Cover store.Write error branches — priority=H, evidence=scout direct, source=scout

## deferred (carry to NEXT cycle's carryoverTodos)
- ledger-seal-io-coverage: Cover writeSegment branches — priority=M, defer_reason=package variety; last worked cycle 318
- cmd-evolve-handlers-coverage: Raise cmd/evolve from 64.4% — priority=L, defer_reason=large scope

## dropped (rejected with reason)
- cycle-311-failed-scout: Bridge artifact timeout — reason=stale; infrastructure transient failure
- cycle-319-failed-scout: Tree-diff leak — reason=stale; fixed in a01d666a

## Rationale
Single coverage floor this cycle for package variety.
`

func TestProjectDecisionJSON_ParsesSupersededSection(t *testing.T) {
	report := realReport + `
## superseded (retire an inbox item by id)
- loop-self-prioritize-unmet-fleet-concurrency: shipped as recover-ship-fleet-starvation-observer in cycle 544
- other-orphan: shipped as x in cycle 500
- loop-self-prioritize-unmet-fleet-concurrency: dup line, must be dropped
`
	body, err := ProjectDecisionJSON(report, 548)
	if err != nil {
		t.Fatalf("ProjectDecisionJSON error: %v", err)
	}
	var got projectedDecision
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("projected JSON invalid: %v\n%s", err, body)
	}
	want := []string{"loop-self-prioritize-unmet-fleet-concurrency", "other-orphan"}
	if len(got.Superseded) != len(want) {
		t.Fatalf("Superseded=%v, want %v (deduped, order-preserving)", got.Superseded, want)
	}
	for i := range want {
		if got.Superseded[i] != want[i] {
			t.Fatalf("Superseded[%d]=%q, want %q", i, got.Superseded[i], want[i])
		}
	}

	base, _ := ProjectDecisionJSON(realReport, 322)
	var noSec projectedDecision
	_ = json.Unmarshal(base, &noSec)
	if noSec.Superseded == nil {
		t.Errorf("absent superseded section marshaled to null, want []")
	}
	if len(noSec.Superseded) != 0 {
		t.Errorf("absent superseded section = %v, want empty", noSec.Superseded)
	}
}

func TestProjectDecisionJSON_ParsesAllSections(t *testing.T) {
	body, err := ProjectDecisionJSON(realReport, 322)
	if err != nil {
		t.Fatalf("ProjectDecisionJSON error: %v", err)
	}

	var got projectedDecision
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("projected JSON is not valid: %v\n%s", err, body)
	}

	if got.Cycle != 322 {
		t.Errorf("cycle = %d, want 322", got.Cycle)
	}
	if len(got.TopN) != 1 || got.TopN[0].ID != "modelcatalog-write-error-paths" {
		t.Fatalf("top_n = %+v, want 1 item modelcatalog-write-error-paths", got.TopN)
	}
	if got.TopN[0].Action == "" {
		t.Errorf("top_n[0].action is empty — the {action} text must be projected")
	}
	wantDeferred := []string{"ledger-seal-io-coverage", "cmd-evolve-handlers-coverage"}
	if len(got.Deferred) != len(wantDeferred) {
		t.Fatalf("deferred = %+v, want %v", got.Deferred, wantDeferred)
	}
	for i, w := range wantDeferred {
		if got.Deferred[i].ID != w {
			t.Errorf("deferred[%d].id = %q, want %q", i, got.Deferred[i].ID, w)
		}
	}
	wantDropped := []string{"cycle-311-failed-scout", "cycle-319-failed-scout"}
	if len(got.Dropped) != len(wantDropped) {
		t.Fatalf("dropped = %+v, want %v", got.Dropped, wantDropped)
	}
	if got.Dropped[0].Reason == "" {
		t.Errorf("dropped[0].reason is empty — the reason= tail must be projected")
	}
}

func TestProjectDecisionJSON_ExtractIDsPromotesOnlyTopN(t *testing.T) {
	body, err := ProjectDecisionJSON(realReport, 322)
	if err != nil {
		t.Fatalf("ProjectDecisionJSON error: %v", err)
	}
	var d struct {
		TopN        []struct{ ID string } `json:"top_n"`
		SkipShipped []struct {
			TaskID string `json:"task_id"`
		} `json:"skip_shipped"`
		Deferred []struct{ ID string } `json:"deferred"`
		Dropped  []struct{ ID string } `json:"dropped"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(d.SkipShipped) != 0 {
		t.Errorf("skip_shipped must be empty in a projection (needs Step-0a git-log judgment), got %d", len(d.SkipShipped))
	}
	if len(d.Deferred) == 0 || len(d.Dropped) == 0 {
		t.Fatal("fixture invalid: deferred/dropped not parsed")
	}
	deferredDropped := map[string]bool{}
	for _, e := range d.Deferred {
		deferredDropped[e.ID] = true
	}
	for _, e := range d.Dropped {
		deferredDropped[e.ID] = true
	}
	for _, e := range d.TopN {
		if deferredDropped[e.ID] {
			t.Errorf("top_n id %q also appears in deferred/dropped — sections must be disjoint", e.ID)
		}
	}
}

func TestProjectDecisionJSON_SkipsMalformedIDs(t *testing.T) {
	report := `# Triage Decision — Cycle 5

## top_n (commit to THIS cycle)
- good-task-id: a real item — priority=H
- This is free-form prose with no id colon structure at all
- Another note: that looks structured but the id has spaces

## deferred

## dropped
`
	body, err := ProjectDecisionJSON(report, 5)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var got projectedDecision
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got.TopN) != 1 || got.TopN[0].ID != "good-task-id" {
		t.Fatalf("top_n = %+v, want only the slug-id item good-task-id", got.TopN)
	}
}

func TestProjectDecisionJSON_EmptySectionsYieldEmptyArraysNotNull(t *testing.T) {
	report := `# Triage Decision — Cycle 9

## top_n (commit to THIS cycle)

## deferred

## dropped
`
	body, err := ProjectDecisionJSON(report, 9)
	if err != nil {
		t.Fatalf("ProjectDecisionJSON error: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, body)
	}
	for _, field := range []string{"top_n", "deferred", "dropped"} {
		v, ok := raw[field]
		if !ok {
			t.Fatalf("%s field missing from projected JSON entirely", field)
		}
		if string(v) != "[]" {
			t.Errorf("%s = %s, want [] for an artifact whose section has no items — nil Go slices must be initialized before marshalling, never left to serialize as null", field, v)
		}
	}
}

func TestProjectDecisionJSON_SectionsEntirelyAbsent_StillYieldEmptyArrays(t *testing.T) {
	report := "# Triage Decision — Cycle 9\n\nNo top_n/deferred/dropped sections at all.\n"
	body, err := ProjectDecisionJSON(report, 9)
	if err != nil {
		t.Fatalf("ProjectDecisionJSON error: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, body)
	}
	for _, field := range []string{"top_n", "deferred", "dropped"} {
		if v := string(raw[field]); v == "null" {
			t.Errorf("%s = null, want [] even when the section heading is entirely absent from the artifact", field)
		}
	}
}

func TestProjectDecisionJSON_FloorFieldsOmitted(t *testing.T) {
	body, err := ProjectDecisionJSON(realReport, 322)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := raw["committed_floors"]; ok {
		t.Error("committed_floors must be OMITTED so floor readers fall back to prose")
	}
	if _, ok := raw["deferred_floors"]; ok {
		t.Error("deferred_floors must be OMITTED so floor readers fall back to prose")
	}
	dir := t.TempDir()
	p := dir + "/triage-decision.json"
	if err := os.WriteFile(p, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, declared, _ := ReadDeclaredFloors(p); declared {
		t.Error("a projected companion must NOT declare floors (prose fallback must stay active)")
	}
}
