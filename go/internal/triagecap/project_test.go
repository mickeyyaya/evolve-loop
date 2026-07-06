package triagecap

import (
	"encoding/json"
	"os"
	"testing"
)

// realReport mirrors a production triage-report.md (cycle-322 shape): the
// canonical headings, the "- {id}: {action} — metadata" item format, and a
// dropped section with reason= tails.
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

// TestProjectDecisionJSON_ParsesSupersededSection pins that a `## superseded`
// section projects into the top-level "superseded" array (deduped, order-
// preserving) which ship's inboxmover.ReconcileSuperseded consumes, and that an
// absent section yields an empty (never null) array.
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

	// Absent section → empty array, never null (consumer expects an array).
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

// TestProjectDecisionJSON_ExtractIDsPromotesOnlyTopN is the load-bearing safety
// property: a projected companion must promote (= move out of inbox) ONLY the
// top_n ids, never the deferred/dropped ids. deferred items carry to the next
// cycle; dropped items were rejected — promoting either would silently lose an
// unresolved inbox item. The projection emits an EMPTY skip_shipped, so the
// union extractIDs walks is exactly top_n.
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
	// Deferred/dropped ids must be parsed (proves the test fixture is real) but
	// must NOT leak into a promotion-eligible field.
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

// TestProjectDecisionJSON_SkipsMalformedIDs guards against a free-form prose
// line producing a bogus id that would promote (delete) a non-existent inbox
// item. Only kebab-slug leading tokens are accepted.
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

// TestProjectDecisionJSON_EmptySectionsYieldEmptyArraysNotNull pins the JSON
// contract: an artifact whose top_n/deferred/dropped sections are all present
// but carry zero list items must still marshal each field as [], never null.
// project.go:66-72's projectedDecision struct fields start as nil Go slices
// and are only appended to when parseItems finds a real item — a report with
// nothing to project today leaves all three nil, and json.MarshalIndent
// serializes a nil slice as null. The consumer (ship/postship.go:169) reads
// this companion; a null top_n is a live regression once disjoint packing
// (triage-fleet-width-disjoint-topn) can legitimately narrow top_n to zero.
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

// TestProjectDecisionJSON_SectionsEntirelyAbsent_StillYieldEmptyArrays covers
// the OTHER null-producing path (section headings absent altogether, not just
// empty) — sectionBody returns ok=false and the field is never touched, same
// nil-slice-to-null failure mode as the present-but-empty case above.
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

// TestProjectDecisionJSON_FloorFieldsOmitted pins that the projection never
// emits committed_floors/deferred_floors — their absence is what makes the
// floor readers fall back to the prose counter, keeping gate behaviour
// identical to the no-companion production baseline.
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
	// committed_floors absent ⇒ ReadDeclaredFloors reports not-declared.
	// (Companion written to a temp file to exercise the real reader.)
	dir := t.TempDir()
	p := dir + "/triage-decision.json"
	if err := os.WriteFile(p, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, declared, _ := ReadDeclaredFloors(p); declared {
		t.Error("a projected companion must NOT declare floors (prose fallback must stay active)")
	}
}
