package inboxbatch

import "testing"

func TestIsOperatorState(t *testing.T) {
	cases := []struct {
		name string
		item Item
		want bool
	}{
		{"evolve-only state mutation", Item{Class: "pipeline-architecture", Files: []string{".evolve/inbox/", ".evolve/state.json"}}, true},
		{"source file disqualifies", Item{Class: "pipeline-architecture", Files: []string{".evolve/state.json", "go/internal/core/loop.go"}}, false},
		{"all source", Item{Class: "pipeline-architecture", Files: []string{"go/internal/core/"}}, false},
		{"empty file list", Item{Class: "pipeline-architecture"}, false},
		{"wrong class", Item{Class: "task-contract-design", Files: []string{".evolve/state.json"}}, false},
		{"no class", Item{Files: []string{".evolve/state.json"}}, false},
		{"lookalike prefix", Item{Class: "pipeline-architecture", Files: []string{".evolvex/state.json"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsOperatorState(tc.item); got != tc.want {
				t.Errorf("IsOperatorState(%+v) = %v, want %v", tc.item, got, tc.want)
			}
		})
	}
}

// TestItemClassRoundTrip pins the json tag itself: a field added without
// `json:"class"` leaves Class empty here.
func TestItemClassRoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeItem(t, dir, "a.json", `{"id":"a","class":"pipeline-architecture"}`)
	writeItem(t, dir, "b.json", `{"id":"b"}`)

	items, warnings, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("LoadDir warned on well-formed items: %v", warnings)
	}
	if len(items) != 2 {
		t.Fatalf("loaded %d items, want 2", len(items))
	}
	if items[0].Class != "pipeline-architecture" {
		t.Errorf("items[0].Class = %q, want %q", items[0].Class, "pipeline-architecture")
	}
	if items[1].Class != "" {
		t.Errorf("items[1].Class = %q, want \"\" (absent class is tolerated, not defaulted)", items[1].Class)
	}
}
