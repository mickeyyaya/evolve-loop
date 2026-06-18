package dag

import (
	"reflect"
	"testing"
)

func TestLevels(t *testing.T) {
	cases := []struct {
		name  string
		nodes []string
		deps  map[string][]string
		want  [][]string
	}{
		{
			name:  "independent nodes form one concurrent wave",
			nodes: []string{"c", "a", "b"},
			deps:  nil,
			want:  [][]string{{"a", "b", "c"}}, // sorted within the level
		},
		{
			name:  "linear chain is one node per wave",
			nodes: []string{"a", "b", "c"},
			deps:  map[string][]string{"b": {"a"}, "c": {"b"}},
			want:  [][]string{{"a"}, {"b"}, {"c"}},
		},
		{
			name:  "diamond: middle pair runs concurrently",
			nodes: []string{"a", "b", "c", "d"},
			deps:  map[string][]string{"b": {"a"}, "c": {"a"}, "d": {"b", "c"}},
			want:  [][]string{{"a"}, {"b", "c"}, {"d"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Levels(tc.nodes, tc.deps)
			if err != nil {
				t.Fatalf("Levels: unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Levels = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLevels_Errors(t *testing.T) {
	cases := []struct {
		name  string
		nodes []string
		deps  map[string][]string
	}{
		{"cycle", []string{"a", "b"}, map[string][]string{"a": {"b"}, "b": {"a"}}},
		{"self-dependency", []string{"a"}, map[string][]string{"a": {"a"}}},
		{"dangling ref", []string{"a"}, map[string][]string{"a": {"ghost"}}},
		{"dep key not a node", []string{"a"}, map[string][]string{"b": {"a"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Levels(tc.nodes, tc.deps); err == nil {
				t.Errorf("Levels(%v, %v): expected error, got nil", tc.nodes, tc.deps)
			}
		})
	}
}

func TestFlatten(t *testing.T) {
	levels := [][]string{{"a"}, {"b", "c"}, {"d"}}
	want := []string{"a", "b", "c", "d"}
	if got := Flatten(levels); !reflect.DeepEqual(got, want) {
		t.Errorf("Flatten = %v, want %v", got, want)
	}
}
