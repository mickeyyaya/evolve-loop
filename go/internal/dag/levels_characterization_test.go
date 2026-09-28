package dag

import (
	"reflect"
	"testing"
)

func TestLevels_ErrorTextAndNilLevels(t *testing.T) {
	cases := []struct {
		name    string
		nodes   []string
		deps    map[string][]string
		wantErr string
	}{
		{"self-dependency", []string{"a"}, map[string][]string{"a": {"a"}}, `dag: node "a" depends on itself`},
		{"dangling ref", []string{"a"}, map[string][]string{"a": {"ghost"}}, `dag: node "a" depends on unknown node "ghost"`},
		{"dep key not a node", []string{"a"}, map[string][]string{"b": {"a"}}, `dag: dependency key "b" is not a known node`},
		{"cycle beside a leveled node", []string{"a", "b", "c"}, map[string][]string{"b": {"c"}, "c": {"b"}}, "dag: graph has a cycle (1 of 3 nodes leveled)"},
		{"whole-graph cycle", []string{"a", "b"}, map[string][]string{"a": {"b"}, "b": {"a"}}, "dag: graph has a cycle (0 of 2 nodes leveled)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Levels(tc.nodes, tc.deps)
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("Levels(%v, %v) error = %v, want %q", tc.nodes, tc.deps, err, tc.wantErr)
			}
			if got != nil {
				t.Errorf("Levels(%v, %v) levels = %v, want nil on error", tc.nodes, tc.deps, got)
			}
		})
	}
}

func TestLevels_DuplicateNodeLevelsOnce(t *testing.T) {
	got, err := Levels([]string{"a", "a"}, nil)
	if err != nil {
		t.Fatalf("Levels([a a], nil): unexpected error: %v", err)
	}
	if want := [][]string{{"a"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("Levels([a a], nil) = %v, want %v", got, want)
	}
}

func TestLevels_EmptyGraphIsNil(t *testing.T) {
	got, err := Levels(nil, nil)
	if err != nil {
		t.Fatalf("Levels(nil, nil): unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("Levels(nil, nil) = %#v, want nil", got)
	}
}
