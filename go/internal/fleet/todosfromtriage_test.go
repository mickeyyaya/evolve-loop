package fleet

// todosfromtriage_test.go — test-amplification for cycle 553's newly EXPORTED
// TodosFromTriage (extracted from PlanFromTriage as a pure refactor so the
// rolling pool dispatch path rolls the SAME backlog the wave path
// partitions; see build-report.md "New Surface" + triageplan.go's doc
// comment surfaced via `go doc`). Black-box against the spec only: before
// this cycle the function was unexported and only ever exercised indirectly
// through PlanFromTriage's returned CycleSpecs; this file pins its contract
// DIRECTLY at the new exported boundary. Fixtures mirror the JSON shapes
// already established by triageplan_test.go / triageplan_amplify_test.go
// (pre-existing, unmodified this cycle) so the parse-schema assumptions are
// grounded in prior-precedent tests, not guessed.

import (
	"encoding/json"
	"fmt"
	"testing"
)

// todoIDs collects the distinct todo IDs TodosFromTriage returned.
func todoIDs(todos []Todo) map[string]bool {
	ids := map[string]bool{}
	for _, td := range todos {
		ids[td.ID] = true
	}
	return ids
}

// TestTodosFromTriage_FloorsBecomeDistinctTodos (positive): committed_floors
// entries must surface as one Todo per distinct id — the same floors
// TestPlanFromTriage_DisjointScopesAcrossLanes partitions into lane specs,
// but pinned here directly against the raw Todo backlog.
func TestTodosFromTriage_FloorsBecomeDistinctTodos(t *testing.T) {
	decisionJSON := []byte(`{"committed_floors":["bridge","core","audit"]}`)
	todos, err := TodosFromTriage(decisionJSON, nil)
	if err != nil {
		t.Fatalf("TodosFromTriage returned error: %v", err)
	}
	ids := todoIDs(todos)
	if len(ids) != 3 {
		t.Fatalf("got %d distinct todo ids, want 3: %v", len(ids), ids)
	}
	for _, want := range []string{"bridge", "core", "audit"} {
		if !ids[want] {
			t.Errorf("missing todo id %q in %v", want, ids)
		}
	}
}

// TestTodosFromTriage_CardPackagesFallbackWhenFloorsAbsent (positive):
// mirrors TestPlanFromTriage_FallsBackToCardPackagesWhenFloorsAbsent's
// fixture, pinned directly against TodosFromTriage.
func TestTodosFromTriage_CardPackagesFallbackWhenFloorsAbsent(t *testing.T) {
	todos, err := TodosFromTriage([]byte(`{}`), []string{"core", "audit"})
	if err != nil {
		t.Fatalf("TodosFromTriage returned error: %v", err)
	}
	ids := todoIDs(todos)
	if !ids["core"] || !ids["audit"] || len(ids) != 2 {
		t.Errorf("todo ids = %v, want exactly {core, audit} (card fallback when floors absent)", ids)
	}
}

// TestTodosFromTriage_FloorsTakePrecedenceOverCards (negative/precedence):
// mirrors TestPlanFromTriage_FloorsTakePrecedenceOverCards — cards must never
// merge into a floors-derived backlog.
func TestTodosFromTriage_FloorsTakePrecedenceOverCards(t *testing.T) {
	todos, err := TodosFromTriage([]byte(`{"committed_floors":["bridge"]}`), []string{"core", "audit"})
	if err != nil {
		t.Fatalf("TodosFromTriage returned error: %v", err)
	}
	ids := todoIDs(todos)
	if len(ids) != 1 || !ids["bridge"] {
		t.Errorf("todo ids = %v, want exactly {bridge} — cards must not merge into a floors-derived backlog", ids)
	}
}

// TestTodosFromTriage_DuplicateFloorsCollapseToDistinctTodos (edge): repeated
// floor ids must collapse to ONE Todo per distinct id, not one Todo per raw
// entry — mirrors TestPlanFromTriage_DuplicateFloorsNeverOverSchedule but
// additionally asserts len(todos) itself (not just the partitioned spec
// count), which the spec-level test could not distinguish from a dedup that
// happened later in PlanCycles instead of in TodosFromTriage.
func TestTodosFromTriage_DuplicateFloorsCollapseToDistinctTodos(t *testing.T) {
	todos, err := TodosFromTriage([]byte(`{"committed_floors":["core","core","audit","core"]}`), nil)
	if err != nil {
		t.Fatalf("TodosFromTriage returned error: %v", err)
	}
	ids := todoIDs(todos)
	if len(ids) != 2 || !ids["core"] || !ids["audit"] {
		t.Errorf("todo ids = %v, want exactly {core, audit} (duplicates collapse to one Todo per distinct id)", ids)
	}
	if len(todos) != len(ids) {
		t.Errorf("len(todos) = %d but %d distinct ids — TodosFromTriage returned duplicate Todo entries for the same id", len(todos), len(ids))
	}
}

// TestTodosFromTriage_MalformedJSONRejectsWithNoTodos (negative): truncated
// JSON must reject, never guess — mirrors
// TestPlanFromTriage_MalformedDecisionJSON_RejectsNotGuesses.
func TestTodosFromTriage_MalformedJSONRejectsWithNoTodos(t *testing.T) {
	todos, err := TodosFromTriage([]byte(`{"committed_floors":[`), []string{"core"})
	if err == nil {
		t.Fatalf("TodosFromTriage(malformed) returned nil error — want an explicit parse error, never a silent guess")
	}
	if len(todos) != 0 {
		t.Errorf("TodosFromTriage(malformed) returned %d todos alongside the error, want 0", len(todos))
	}
}

// TestTodosFromTriage_WrongTypeFieldsRejected (negative): mirrors
// TestPlanFromTriage_WrongTypeDecisionFieldsRejected — wrong-typed decision
// JSON must reject rather than silently falling back to cards (masking a
// corrupted triage artifact).
func TestTodosFromTriage_WrongTypeFieldsRejected(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"floors-is-a-string", `{"committed_floors":"core"}`},
		{"floors-is-a-number-array", `{"committed_floors":[1,2,3]}`},
		{"document-is-a-bare-number", `42`},
		{"document-is-an-array", `["core","audit"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			todos, err := TodosFromTriage([]byte(tc.in), []string{"core"})
			if err == nil {
				t.Fatalf("TodosFromTriage(%s) returned nil error — wrong-typed decision JSON must reject", tc.in)
			}
			if len(todos) != 0 {
				t.Errorf("TodosFromTriage(%s) returned %d todos alongside the error, want 0", tc.in, len(todos))
			}
		})
	}
}

// TestTodosFromTriage_DegenerateBytesNeverPanicsOrPartialErrors (edge/OOD):
// mirrors TestPlanFromTriage_DegenerateDecisionBytesFailSafe — empty/nil/null
// decision bytes must never panic and must never return both an error AND
// non-empty todos.
func TestTodosFromTriage_DegenerateBytesNeverPanicsOrPartialErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []byte
	}{
		{"empty-bytes", []byte{}},
		{"nil-bytes", nil},
		{"bare-null", []byte(`null`)},
		{"floors-null", []byte(`{"committed_floors":null}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			todos, err := TodosFromTriage(tc.in, []string{"core"})
			if err != nil {
				if len(todos) != 0 {
					t.Errorf("error return carried %d todos, want 0 (never both)", len(todos))
				}
				return
			}
			ids := todoIDs(todos)
			if len(ids) != 1 || !ids["core"] {
				t.Errorf("todo ids = %v, want exactly {core} (card fallback on the nil-error path)", ids)
			}
		})
	}
}

// TestTodosFromTriage_LargeScaleAllFloorsSurviveDistinctly (limit/large-scale):
// mirrors TestPlanFromTriage_LargeScaleAllFloorsScheduledDisjoint (100
// floors) but widened to 200 and pinned directly against the raw Todo count
// (not the post-partition spec count) — no floor may be silently dropped or
// merged during the parse itself.
func TestTodosFromTriage_LargeScaleAllFloorsSurviveDistinctly(t *testing.T) {
	floors := make([]string, 200)
	for i := range floors {
		floors[i] = fmt.Sprintf("pkg%03d", i)
	}
	raw, err := json.Marshal(map[string][]string{"committed_floors": floors})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	todos, err := TodosFromTriage(raw, nil)
	if err != nil {
		t.Fatalf("TodosFromTriage returned error: %v", err)
	}
	ids := todoIDs(todos)
	if len(ids) != 200 {
		t.Errorf("got %d distinct todo ids, want 200 — no floor may be silently dropped", len(ids))
	}
}

// TestTodosFromTriage_TopNCardsBecomeTodos (positive): mirrors
// TestPlanFromTriage_ProductionFixtureTopNOnlyFallback — a real triage-
// decision.json shape (top_n cards, no committed_floors) must still parse
// into a non-empty Todo backlog.
func TestTodosFromTriage_TopNCardsBecomeTodos(t *testing.T) {
	decisionJSON := []byte(`{
		"cycle": 464,
		"top_n": [
			{"id": "fleet-policy-block", "action": "Add FleetPolicy block."},
			{"id": "fleet-policy-docs", "action": "Document the fleet block."}
		],
		"deferred": [{"id": "cycle-366-failed-ship"}],
		"dropped": null,
		"projected_by_orchestrator": true
	}`)
	todos, err := TodosFromTriage(decisionJSON, nil)
	if err != nil {
		t.Fatalf("TodosFromTriage(top_n-only) returned error: %v, want nil", err)
	}
	ids := todoIDs(todos)
	for _, want := range []string{"fleet-policy-block", "fleet-policy-docs"} {
		if !ids[want] {
			t.Errorf("todo ids = %v, missing top_n card id %q", ids, want)
		}
	}
}
