package triagecap

import "testing"

// topn_width_test.go pins the fleet-width-aware, file-disjoint top_n
// selection (inbox triage-supply-disjoint-topn-for-fleet-width, weight 0.94):
// cycle-503 triage committed exactly 1 top_n task and starved the fleet wave
// planner of the >=2 disjoint tasks it needs to fan out 2 concurrent lanes.
// SelectFleetWidthTopN(candidates, count) is the new SSOT: it greedily packs
// the highest-weight candidates into up to `count` mutually FILE-DISJOINT
// lanes (delegating to fleet.Partition's greedy-ownership algorithm — see
// go/internal/fleet/partition.go) and returns one representative per
// non-empty lane, so the returned set is always safe to fan out 1:1 into
// concurrent `evolve cycle run` lanes without a cross-lane file collision.
//
// count<2 must reproduce today's single-focus behavior byte-identically
// (inbox acceptance #3): exactly the single highest-weight candidate,
// independent of file overlap.

// TestTopN_FleetWidthAware_ProducesDisjointSet is the baseline acceptance
// case (inbox acceptance #1): 2 already-disjoint candidates + fleet.count=2
// must yield both, and they must be mutually file-disjoint.
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

// TestTopN_FleetWidthAware_ThreeDisjointTwoOverlapping_PacksTwoDisjoint is
// the exact regression fixture named in the inbox item's acceptance spec: "3
// disjoint + 2 overlapping items, fleet.count=2 -> triage top_n has >=2
// mutually-disjoint items". task-d/task-e each overlap one of the two
// highest-weight disjoint items (task-a/task-b); the greedy pack must still
// surface task-a and task-b as the two lane representatives, not fabricate a
// pairing and not silently drop to 1 lane.
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

// TestTopN_FleetWidthAware_FewerThanCountDisjoint_ReturnsWidestSetNoOverlap
// covers inbox acceptance #2: when the backlog cannot fill `count` disjoint
// lanes (every candidate here shares one file), the widest disjoint set
// (here, 1) is returned — never a fabricated/overlapping pairing that would
// bridge two concurrent lanes into a file collision.
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

// TestTopN_FleetWidthAware_CountOneOrAbsent_PreservesSingleTopNBehavior
// covers inbox acceptance #3: fleet.count=1 (or the zero-value "absent" a
// caller might pass before policy defaulting) must reproduce the pre-fleet
// single-focus selection byte-identically — the single highest-weight
// candidate, regardless of any file overlap among the others.
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

// TestTopN_FleetWidthAware_EmptyCandidates_ReturnsEmptyNoPanic is the OOD
// edge case: an empty backlog must not panic and must return an empty
// selection.
func TestTopN_FleetWidthAware_EmptyCandidates_ReturnsEmptyNoPanic(t *testing.T) {
	if got := SelectFleetWidthTopN(nil, 2); len(got) != 0 {
		t.Errorf("SelectFleetWidthTopN(nil, 2) = %+v, want empty", got)
	}
}

// assertMutuallyDisjoint is the negative-axis guard (adversarial-testing
// SKILL §6): a no-op or overlap-tolerant implementation could satisfy a bare
// length check while still returning two candidates that share a file, which
// would collide two concurrent fleet lanes on the same tree. This fails that
// class of fake-disjoint implementation directly.
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
