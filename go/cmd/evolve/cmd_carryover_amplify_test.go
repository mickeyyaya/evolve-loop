package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCarryoverApplyDecisions_DryRunDoesNotMutateState(t *testing.T) {
	statePath := writeFixtureState(t, "todo-keep-me", "todo-drop-me")
	beforeRaw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read fixture state: %v", err)
	}

	doc := carryoverDecisionsDoc{
		SourceCount: 2,
		Decisions: []carryoverDecisionRow{
			{ID: "todo-keep-me", Decision: "keep", Reason: "still live"},
			{ID: "todo-drop-me", Decision: "drop", Reason: "stale"},
		},
	}
	decisionsPath := writeDecisionsFile(t, doc)

	code := runCarryoverApplyDecisions([]string{"--state", statePath, "--decisions", decisionsPath}, io.Discard, io.Discard)
	if code != 0 {
		t.Fatalf("dry-run (no --apply) returned non-zero exit %d; a plan-only run must succeed", code)
	}

	afterRaw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state after dry-run: %v", err)
	}
	if string(beforeRaw) != string(afterRaw) {
		t.Fatalf("state.json was mutated by a dry-run (no --apply flag) — dry-run must be read-only")
	}
	after := readCarryoverIDs(t, statePath)
	if !after["todo-drop-me"] {
		t.Fatalf("todo-drop-me is gone after a dry-run — --apply is supposed to be mandatory for any mutation")
	}
}

func TestCarryoverApplyDecisions_RejectsFlatArraySchema(t *testing.T) {
	statePath := writeFixtureState(t, "todo-a", "todo-b")
	beforeRaw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read fixture state: %v", err)
	}

	flatArray := `[{"id":"todo-a","decision":"drop","reason":"stale"}]`
	decisionsPath := filepath.Join(t.TempDir(), "decisions.json")
	if err := os.WriteFile(decisionsPath, []byte(flatArray), 0o644); err != nil {
		t.Fatalf("write malformed decisions fixture: %v", err)
	}

	code := runCarryoverApplyDecisions([]string{"--apply", "--state", statePath, "--decisions", decisionsPath}, io.Discard, io.Discard)

	afterRaw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state after malformed apply: %v", err)
	}
	if string(beforeRaw) != string(afterRaw) {
		t.Fatalf("state.json was mutated by a wrong-shape (flat array) decisions file — a malformed schema must never produce a partial or silent apply")
	}
	if code == 0 {
		t.Errorf("runCarryoverApplyDecisions returned 0 for a flat-array decisions file (wrong schema); malformed input should be rejected with a non-zero exit, not silently accepted as an empty no-op plan")
	}
}

func TestCarryoverApplyDecisions_RejectsDuplicateID(t *testing.T) {
	statePath := writeFixtureState(t, "todo-dup")
	beforeRaw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read fixture state: %v", err)
	}

	doc := carryoverDecisionsDoc{
		SourceCount: 1,
		Decisions: []carryoverDecisionRow{
			{ID: "todo-dup", Decision: "keep", Reason: "first row says keep"},
			{ID: "todo-dup", Decision: "drop", Reason: "second row says drop"},
		},
	}
	if verr := validateCarryoverDecisions(doc); verr == nil {
		t.Errorf("validateCarryoverDecisions accepted a decisions file with duplicate id %q and conflicting decisions", "todo-dup")
	}

	code := runCarryoverApplyDecisions([]string{"--apply", "--state", statePath, "--decisions", writeDecisionsFile(t, doc)}, io.Discard, io.Discard)
	if code == 0 {
		t.Errorf("runCarryoverApplyDecisions returned 0 for a decisions file with a duplicated, conflicting id")
	}
	afterRaw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state after duplicate-id apply: %v", err)
	}
	if string(beforeRaw) != string(afterRaw) {
		t.Fatalf("state.json was mutated despite a duplicate-id decisions file")
	}
}

func TestCarryoverApplyDecisions_RejectsInvalidDecisionEnum(t *testing.T) {
	statePath := writeFixtureState(t, "todo-a")
	beforeRaw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read fixture state: %v", err)
	}

	doc := carryoverDecisionsDoc{
		SourceCount: 1,
		Decisions: []carryoverDecisionRow{
			{ID: "todo-a", Decision: "archive", Reason: "not a real enum value"},
		},
	}
	if verr := validateCarryoverDecisions(doc); verr == nil {
		t.Errorf("validateCarryoverDecisions accepted decision=%q (not one of keep|drop|cluster)", "archive")
	}

	code := runCarryoverApplyDecisions([]string{"--apply", "--state", statePath, "--decisions", writeDecisionsFile(t, doc)}, io.Discard, io.Discard)
	if code == 0 {
		t.Errorf("runCarryoverApplyDecisions returned 0 for an invalid decision enum value")
	}
	afterRaw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state after invalid-enum apply: %v", err)
	}
	if string(beforeRaw) != string(afterRaw) {
		t.Fatalf("state.json was mutated despite an invalid-enum decisions file")
	}
}

func TestCarryoverApplyDecisions_RejectsClusterWithoutGroup(t *testing.T) {
	statePath := writeFixtureState(t, "todo-a")
	beforeRaw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read fixture state: %v", err)
	}

	doc := carryoverDecisionsDoc{
		SourceCount: 1,
		Decisions: []carryoverDecisionRow{
			{ID: "todo-a", Decision: "cluster", Reason: "amortised", ClusterGroup: ""},
		},
	}
	if verr := validateCarryoverDecisions(doc); verr == nil {
		t.Errorf("validateCarryoverDecisions accepted a `cluster` row with an empty cluster_group")
	}

	code := runCarryoverApplyDecisions([]string{"--apply", "--state", statePath, "--decisions", writeDecisionsFile(t, doc)}, io.Discard, io.Discard)
	if code == 0 {
		t.Errorf("runCarryoverApplyDecisions returned 0 for a cluster row missing cluster_group")
	}
	afterRaw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state after cluster-without-group apply: %v", err)
	}
	if string(beforeRaw) != string(afterRaw) {
		t.Fatalf("state.json was mutated despite a cluster row missing cluster_group")
	}
}

func TestCarryoverApplyDecisions_UnknownIDIgnoredNotCrash(t *testing.T) {
	statePath := writeFixtureState(t, "todo-real")
	doc := carryoverDecisionsDoc{
		SourceCount: 2,
		Decisions: []carryoverDecisionRow{
			{ID: "todo-real", Decision: "keep", Reason: "still live"},
			{ID: "todo-ghost-not-in-state", Decision: "drop", Reason: "stale, already gone"},
		},
	}
	res, err := applyCarryoverDecisions(statePath, doc)
	if err != nil {
		t.Fatalf("applyCarryoverDecisions errored on a decision id absent from state.json: %v", err)
	}
	surviving := readCarryoverIDs(t, statePath)
	if !surviving["todo-real"] {
		t.Fatalf("todo-real (a `keep` decision) was incorrectly removed")
	}
	if len(surviving) != 1 {
		t.Fatalf("surviving id count = %d, want 1 (unknown ghost id must not appear or affect real entries)", len(surviving))
	}
	t.Logf("unknown-id apply result: %+v", res)
}

func TestCarryoverApplyDecisions_EmptyDecisionsIsNoOp(t *testing.T) {
	statePath := writeFixtureState(t, "todo-a", "todo-b")
	doc := carryoverDecisionsDoc{SourceCount: 0, Decisions: nil}

	res, err := applyCarryoverDecisions(statePath, doc)
	if err != nil {
		t.Fatalf("applyCarryoverDecisions errored on an empty decisions array: %v", err)
	}
	if res.Dropped != 0 || res.Clustered != 0 {
		t.Errorf("empty decisions produced non-zero drop/cluster: %+v", res)
	}
	surviving := readCarryoverIDs(t, statePath)
	if len(surviving) != 2 {
		t.Errorf("empty decisions changed the live count: got %d ids, want 2 unchanged", len(surviving))
	}
}

func TestCarryoverApplyDecisions_CeilingReportedNotEnforced(t *testing.T) {
	const n = 40
	ids := make([]string, 0, n)
	doc := carryoverDecisionsDoc{SourceCount: n}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("todo-keep-%02d", i)
		ids = append(ids, id)
		doc.Decisions = append(doc.Decisions, carryoverDecisionRow{ID: id, Decision: "keep", Reason: "all live"})
	}
	statePath := writeFixtureState(t, ids...)

	res, err := applyCarryoverDecisions(statePath, doc)
	if err != nil {
		t.Fatalf("applyCarryoverDecisions errored when survivors exceed the ceiling (must WARN, not fail): %v", err)
	}
	if res.After <= carryoverApplyCeiling {
		t.Fatalf("fixture setup bug: After=%d must exceed ceiling=%d to exercise the over-ceiling path", res.After, carryoverApplyCeiling)
	}
	surviving := readCarryoverIDs(t, statePath)
	if len(surviving) != n {
		t.Errorf("surviving id count = %d, want %d (all `keep`; the ceiling must not force extra drops)", len(surviving), n)
	}
}

func TestCarryoverApplyDecisions_LargeScaleConverges(t *testing.T) {
	const n = 3000
	const surviveEvery = 50
	ids := make([]string, 0, n)
	doc := carryoverDecisionsDoc{SourceCount: n}
	wantSurvivors := 0
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("todo-scale-%05d", i)
		ids = append(ids, id)
		decision := "drop"
		if i%surviveEvery == 0 {
			decision = "keep"
			wantSurvivors++
		}
		doc.Decisions = append(doc.Decisions, carryoverDecisionRow{ID: id, Decision: decision, Reason: "bulk fixture"})
	}
	statePath := writeFixtureState(t, ids...)

	res, err := applyCarryoverDecisions(statePath, doc)
	if err != nil {
		t.Fatalf("applyCarryoverDecisions errored at scale (n=%d): %v", n, err)
	}
	if res.Before != n {
		t.Errorf("Before = %d, want %d", res.Before, n)
	}
	if res.After != wantSurvivors {
		t.Errorf("After = %d, want %d", res.After, wantSurvivors)
	}
	surviving := readCarryoverIDs(t, statePath)
	if len(surviving) != wantSurvivors {
		t.Errorf("surviving id count = %d, want %d", len(surviving), wantSurvivors)
	}
}
