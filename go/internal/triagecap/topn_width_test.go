package triagecap

import "testing"

func TestTopN_FleetWidthAware_ProducesDisjointSet(t *testing.T) {
	candidates := []FleetCandidate{
		{ID: "task-a", Weight: 0.9, Files: []string{"go/internal/pkga/a.go"}},
		{ID: "task-b", Weight: 0.8, Files: []string{"go/internal/pkgb/b.go"}},
	}
	got := SelectFleetWidthTopN(candidates, 2)
	if len(got) != 2 {
		t.Fatalf("SelectFleetWidthTopN(2 disjoint candidates, count=2) = %d item(s), want 2: %+v", len(got), got)
	}
	assertMutuallyDisjoint(t, got)
}

func TestTopN_FleetWidthAware_ThreeDisjointTwoOverlapping_PacksTwoDisjoint(t *testing.T) {
	candidates := []FleetCandidate{
		{ID: "task-a", Weight: 0.95, Files: []string{"go/internal/pkga/a.go"}},
		{ID: "task-b", Weight: 0.90, Files: []string{"go/internal/pkgb/b.go"}},
		{ID: "task-d-overlaps-a", Weight: 0.85, Files: []string{"go/internal/pkga/a.go"}},
		{ID: "task-e-overlaps-b", Weight: 0.80, Files: []string{"go/internal/pkgb/b.go"}},
		{ID: "task-c", Weight: 0.60, Files: []string{"go/internal/pkgc/c.go"}},
	}
	got := SelectFleetWidthTopN(candidates, 2)
	if len(got) != 2 {
		t.Fatalf("top_n = %+v, want exactly 2 mutually-disjoint items for fleet.count=2", got)
	}
	assertMutuallyDisjoint(t, got)
	ids := map[string]bool{}
	for _, c := range got {
		ids[c.ID] = true
	}
	if !ids["task-a"] || !ids["task-b"] {
		t.Errorf("top_n = %+v, want the two highest-weight disjoint representatives task-a and task-b", got)
	}
}

func TestTopN_FleetWidthAware_FewerThanCountDisjoint_ReturnsWidestSetNoOverlap(t *testing.T) {
	candidates := []FleetCandidate{
		{ID: "task-x", Weight: 0.9, Files: []string{"go/internal/shared/s.go"}},
		{ID: "task-y", Weight: 0.7, Files: []string{"go/internal/shared/s.go"}},
		{ID: "task-z", Weight: 0.5, Files: []string{"go/internal/shared/s.go"}},
	}
	got := SelectFleetWidthTopN(candidates, 2)
	if len(got) != 1 {
		t.Fatalf("all 3 candidates share one file — the widest disjoint set is 1, got %d: %+v", len(got), got)
	}
	if got[0].ID != "task-x" {
		t.Errorf("got %+v, want only the single highest-weight candidate task-x, never a fabricated overlap", got)
	}
}

func TestTopN_FleetWidthAware_CountOneOrAbsent_PreservesSingleTopNBehavior(t *testing.T) {
	candidates := []FleetCandidate{
		{ID: "task-low", Weight: 0.6, Files: []string{"go/internal/a/a.go"}},
		{ID: "task-top", Weight: 0.9, Files: []string{"go/internal/b/b.go"}},
		{ID: "task-mid", Weight: 0.75, Files: []string{"go/internal/c/c.go"}},
	}
	for _, count := range []int{0, 1} {
		got := SelectFleetWidthTopN(candidates, count)
		if len(got) != 1 || got[0].ID != "task-top" {
			t.Errorf("SelectFleetWidthTopN(count=%d) = %+v, want exactly the single highest-weight candidate task-top (legacy single-focus behavior)", count, got)
		}
	}
}

func TestTopN_FleetWidthAware_EmptyCandidates_ReturnsEmptyNoPanic(t *testing.T) {
	if got := SelectFleetWidthTopN(nil, 2); len(got) != 0 {
		t.Errorf("SelectFleetWidthTopN(nil, 2) = %+v, want empty", got)
	}
}

func assertMutuallyDisjoint(t *testing.T, got []FleetCandidate) {
	t.Helper()
	seen := map[string]string{} // file -> owning candidate id
	for _, c := range got {
		for _, f := range c.Files {
			if owner, ok := seen[f]; ok {
				t.Fatalf("file %q claimed by both %q and %q — top_n is not mutually file-disjoint", f, owner, c.ID)
			}
			seen[f] = c.ID
		}
	}
}
