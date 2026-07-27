package inboxbatch

// rules_area_floor_test.go — the discriminative FLOOR on file-area grouping.
//
// fileAreaRule already has a discriminative CEILING (hubAreaMaxItems): an area
// referenced by more than 5 items binds nothing, because "go/internal/core
// appears in half the real backlog" is not a signal. The same argument applies
// from below and was missing: fileArea caps at areaDepth=3 segments but had no
// minimum, so every persona file in the repo collapsed to the single area
// "agents" and every top-level skill file to "skills". Those are bags of
// unrelated files, not units of work — one worktree/build/audit cannot
// meaningfully carry "everything that touches agents/".
//
// Live consequence measured on the 84-item backlog (2026-07-27): two "agents"
// edges chained three unrelated campaigns —
//
//	agents: chronicle-s7a-historian-shadow [chronicle-2026-07]
//	     <-> inbox-console-worklist-view   [pipeline-integrity]
//	     <-> acs-metapredicate-suite-scope [convergence-2026-07]
//
// producing one 43-item cluster (51% of the backlog) chunked into 11 batches
// each marked "run the previous batch first" — an 11-cycle serialized chain,
// and the same mega-cluster pathology hubAreaMaxItems was introduced to kill.
// With the floor: largest cluster 27 (32%), and the convergence campaign splits
// into its own clean 11-item cluster.

import "testing"

// TestFileArea_RequiresMinimumDepthToBeDiscriminative pins both directions of
// the floor: a top-level bag yields no area, a real package/topic dir does.
func TestFileArea_RequiresMinimumDepthToBeDiscriminative(t *testing.T) {
	for _, tc := range []struct {
		name, file, want string
	}{
		// Below the floor — bags of unrelated files, must bind nothing.
		{"persona-dir-is-a-bag", "agents/evolve-tdd-engineer.md", ""},
		{"skills-root-is-a-bag", "skills/audit.md", ""},
		{"go-root-is-the-whole-codebase", "go/evolve", ""},
		{"bare-dir-reference", "agents/", ""},

		// At or above the floor — genuine units of work, must still bind.
		{"go-package", "go/internal/acssuite/acssuite.go", "go/internal/acssuite"},
		{"named-skill-dir", "skills/audit/SKILL.md", "skills/audit"},
		{"docs-topic-dir", "docs/operations/runtime-reference.md", "docs/operations"},
		{"deep-path-caps-at-areaDepth", "go/internal/bridge/manifests/claude-tmux.json", "go/internal/bridge"},

		// Pre-existing contract: a bare filename has no area at all.
		{"bare-filename", "Makefile", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := fileArea(tc.file); got != tc.want {
				t.Errorf("fileArea(%q) = %q, want %q — a sub-%d-segment area is a bag of unrelated files and must bind nothing",
					tc.file, got, tc.want, minAreaDepth)
			}
		})
	}
}

// TestFileAreaRule_ShallowSharedDirDoesNotBindUnrelatedItems is the behavioral
// consequence: two items whose ONLY commonality is a top-level directory must
// not be grouped. This is the edge that fused three campaigns in the live
// backlog.
func TestFileAreaRule_ShallowSharedDirDoesNotBindUnrelatedItems(t *testing.T) {
	items := []Item{
		{ID: "historian-shadow", Files: []string{"agents/evolve-historian.md"}},
		{ID: "metapredicate-scope", Files: []string{"agents/evolve-tdd-engineer.md"}},
	}
	if edges := (fileAreaRule{}).Edges(items); len(edges) != 0 {
		t.Errorf("fileAreaRule bound two items that share only the top-level %q directory: %+v — "+
			"'both edit some persona' is not a unit of work, and this edge is what chained "+
			"chronicle-2026-07, pipeline-integrity and convergence-2026-07 into one 43-item cluster", "agents", edges)
	}
}

// TestFileAreaRule_RealPackageStillBinds (anti-degenerate): the floor must not
// disable the rule wholesale — a shared Go package is exactly the signal
// fileAreaRule exists to capture.
func TestFileAreaRule_RealPackageStillBinds(t *testing.T) {
	items := []Item{
		{ID: "evidence-tail", Files: []string{"go/internal/acssuite/acssuite.go"}},
		{ID: "predicate-scope", Files: []string{"go/internal/acssuite/rules.go"}},
	}
	edges := (fileAreaRule{}).Edges(items)
	if len(edges) != 1 {
		t.Fatalf("fileAreaRule produced %d edges for two items in the same package, want 1 — "+
			"the depth floor must not disable genuine package grouping: %+v", len(edges), edges)
	}
	if edges[0].Reason != "file-area go/internal/acssuite" {
		t.Errorf("edge reason = %q, want %q (the Reason surfaces to operators as WHY a batch holds together)",
			edges[0].Reason, "file-area go/internal/acssuite")
	}
}
