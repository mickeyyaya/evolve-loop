package fleet

import (
	"encoding/json"
	"fmt"
	"testing"
)

func todoIDs(todos []Todo) map[string]bool {
	ids := map[string]bool{}
	for _, td := range todos {
		ids[td.ID] = true
	}
	return ids
}

func TestTodosFromTriage_FloorsBecomeDistinctTodos(t *testing.T) {
	decisionJSON := []byte(`{"committed_floors":["bridge","core","audit"]}`)
	todos, _, err := TodosFromTriage(decisionJSON, nil, nil)
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

func TestTodosFromTriage_CardPackagesFallbackWhenFloorsAbsent(t *testing.T) {
	todos, _, err := TodosFromTriage([]byte(`{}`), []string{"core", "audit"}, nil)
	if err != nil {
		t.Fatalf("TodosFromTriage returned error: %v", err)
	}
	ids := todoIDs(todos)
	if !ids["core"] || !ids["audit"] || len(ids) != 2 {
		t.Errorf("todo ids = %v, want exactly {core, audit} (card fallback when floors absent)", ids)
	}
}

func TestTodosFromTriage_FloorsTakePrecedenceOverCards(t *testing.T) {
	todos, _, err := TodosFromTriage([]byte(`{"committed_floors":["bridge"]}`), []string{"core", "audit"}, nil)
	if err != nil {
		t.Fatalf("TodosFromTriage returned error: %v", err)
	}
	ids := todoIDs(todos)
	if len(ids) != 1 || !ids["bridge"] {
		t.Errorf("todo ids = %v, want exactly {bridge} — cards must not merge into a floors-derived backlog", ids)
	}
}

func TestTodosFromTriage_DuplicateFloorsCollapseToDistinctTodos(t *testing.T) {
	todos, _, err := TodosFromTriage([]byte(`{"committed_floors":["core","core","audit","core"]}`), nil, nil)
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

func TestTodosFromTriage_MalformedJSONRejectsWithNoTodos(t *testing.T) {
	todos, _, err := TodosFromTriage([]byte(`{"committed_floors":[`), []string{"core"}, nil)
	if err == nil {
		t.Fatalf("TodosFromTriage(malformed) returned nil error — want an explicit parse error, never a silent guess")
	}
	if len(todos) != 0 {
		t.Errorf("TodosFromTriage(malformed) returned %d todos alongside the error, want 0", len(todos))
	}
}

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
			todos, _, err := TodosFromTriage([]byte(tc.in), []string{"core"}, nil)
			if err == nil {
				t.Fatalf("TodosFromTriage(%s) returned nil error — wrong-typed decision JSON must reject", tc.in)
			}
			if len(todos) != 0 {
				t.Errorf("TodosFromTriage(%s) returned %d todos alongside the error, want 0", tc.in, len(todos))
			}
		})
	}
}

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
			todos, _, err := TodosFromTriage(tc.in, []string{"core"}, nil)
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

func TestTodosFromTriage_LargeScaleAllFloorsSurviveDistinctly(t *testing.T) {
	floors := make([]string, 200)
	for i := range floors {
		floors[i] = fmt.Sprintf("pkg%03d", i)
	}
	raw, err := json.Marshal(map[string][]string{"committed_floors": floors})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	todos, _, err := TodosFromTriage(raw, nil, nil)
	if err != nil {
		t.Fatalf("TodosFromTriage returned error: %v", err)
	}
	ids := todoIDs(todos)
	if len(ids) != 200 {
		t.Errorf("got %d distinct todo ids, want 200 — no floor may be silently dropped", len(ids))
	}
}

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
	todos, _, err := TodosFromTriage(decisionJSON, nil, nil)
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
