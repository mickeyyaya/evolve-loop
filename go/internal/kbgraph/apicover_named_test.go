package kbgraph

import "testing"

func TestNamedExports(t *testing.T) {
	var g *Graph
	g, err := Build(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var (
		_ Node       = Node{ID: "n", Kind: "k", Label: "l"}
		_ Edge       = Edge{From: "a", To: "b", Kind: KindRecursAs}
		_ Subgraph   = Subgraph{}
		_ Recurrence = g.Recurrences("none")
	)
	kinds := []string{KindRecursAs, KindHasDefect, KindYielded, KindConnectsTo, KindDependsOn}
	for _, k := range kinds {
		if k == "" {
			t.Error("empty edge kind constant")
		}
	}
	if MaxHops != 2 || MaxNodes != 40 {
		t.Errorf("caps = %d/%d, want 2/40", MaxHops, MaxNodes)
	}
	if _, err := g.Neighbors("cycle:1", 1); err == nil {
		t.Error("Neighbors of an unknown node returned nil error")
	}
}
