package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeCarryoverState(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func carryoverDoc(rows ...carryoverDecisionRow) carryoverDecisionsDoc {
	return carryoverDecisionsDoc{SourceCount: len(rows), Decisions: rows}
}

func TestApplyCarryoverDecisions_AnAbsentStateFileIsAPathError(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "state.json")

	res, err := applyCarryoverDecisions(missing, carryoverDoc(carryoverDecisionRow{ID: "a", Decision: "drop"}))

	if err == nil || !strings.HasPrefix(err.Error(), "read state "+missing+": ") {
		t.Errorf("err = %v, want the read-state error naming the path", err)
	}
	if res != (carryoverApplyResult{}) {
		t.Errorf("res = %+v, want zero", res)
	}
	if _, statErr := os.Stat(missing); !os.IsNotExist(statErr) {
		t.Errorf("a refused apply created %s", missing)
	}
}

func TestApplyCarryoverDecisions_KeepsEntriesThatAreNotObjectsInPlace(t *testing.T) {
	t.Parallel()
	state := writeCarryoverState(t, `{"carryoverTodos":["legacy-string",{"id":"a"},7,{"id":"b"},{"id":"c"}]}`)

	res, err := applyCarryoverDecisions(state, carryoverDoc(
		carryoverDecisionRow{ID: "a", Decision: "drop"},
		carryoverDecisionRow{ID: "c", Decision: "cluster", ClusterGroup: "g"},
		carryoverDecisionRow{ID: "b", Decision: "keep"},
	))

	if err != nil {
		t.Fatal(err)
	}
	if want := (carryoverApplyResult{Before: 5, After: 3, Dropped: 1, Clustered: 1}); res != want {
		t.Errorf("res = %+v, want %+v", res, want)
	}
	raw, _ := os.ReadFile(state)
	var got struct {
		CarryoverTodos []any `json:"carryoverTodos"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	want := []any{"legacy-string", float64(7), map[string]any{"id": "b"}}
	if !reflect.DeepEqual(got.CarryoverTodos, want) {
		t.Errorf("carryoverTodos = %v, want %v", got.CarryoverTodos, want)
	}

	again, err := applyCarryoverDecisions(state, carryoverDoc(carryoverDecisionRow{ID: "a", Decision: "drop"}))
	if err != nil || again != (carryoverApplyResult{Before: 3, After: 3}) {
		t.Errorf("re-apply = %+v, %v: applying the same drop twice must be a no-op", again, err)
	}
}

func TestCarryoverNoOpFastPath(t *testing.T) {
	t.Parallel()
	remove := map[string]string{"x": "drop"}
	cases := []struct {
		name   string
		body   *string
		want   carryoverApplyResult
		wantOK bool
	}{
		{"a malformed state falls through to the locked update", carryoverBody(`{"carryoverTodos":`), carryoverApplyResult{}, false},
		{"an absent state falls through", nil, carryoverApplyResult{}, false},
		{"a matching id falls through", carryoverBody(`{"carryoverTodos":[{"id":"x"}]}`), carryoverApplyResult{}, false},
		{"no match is a no-op", carryoverBody(`{"carryoverTodos":[{"id":"y"},"s",{"id":3}]}`), carryoverApplyResult{Before: 3, After: 3}, true},
		{"an empty state is a no-op", carryoverBody(``), carryoverApplyResult{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "state.json")
			if tc.body != nil {
				path = writeCarryoverState(t, *tc.body)
			}
			got, ok := carryoverNoOpFastPath(path, remove)
			if ok != tc.wantOK || got != tc.want {
				t.Errorf("fastPath = (%+v, %v), want (%+v, %v)", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestApplyCarryoverDecisions_AMalformedStateIsAnError(t *testing.T) {
	t.Parallel()
	state := writeCarryoverState(t, `{"carryoverTodos":`)

	_, err := applyCarryoverDecisions(state, carryoverDoc(carryoverDecisionRow{ID: "a", Decision: "drop"}))

	if err == nil || !strings.Contains(err.Error(), "parse ") {
		t.Errorf("err = %v, want the locked re-read's parse error", err)
	}
	if raw, _ := os.ReadFile(state); string(raw) != `{"carryoverTodos":` {
		t.Errorf("state = %q, want it untouched", raw)
	}
}

func carryoverBody(s string) *string { return &s }
