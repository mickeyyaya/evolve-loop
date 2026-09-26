package inboxbatch

import "testing"

func TestFileArea_RequiresMinimumDepthToBeDiscriminative(t *testing.T) {
	for _, tc := range []struct {
		name, file, want string
	}{
		{"persona-dir-is-a-bag", "agents/evolve-tdd-engineer.md", ""},
		{"skills-root-is-a-bag", "skills/audit.md", ""},
		{"go-root-is-the-whole-codebase", "go/evolve", ""},
		{"bare-dir-reference", "agents/", ""},

		{"go-package", "go/internal/acssuite/acssuite.go", "go/internal/acssuite"},
		{"named-skill-dir", "skills/audit/SKILL.md", "skills/audit"},
		{"docs-topic-dir", "docs/operations/runtime-reference.md", "docs/operations"},
		{"deep-path-caps-at-areaDepth", "go/internal/bridge/manifests/claude-tmux.json", "go/internal/bridge"},

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
