package kbgraph

import "testing"

func TestNamedExports(t *testing.T) {
	var g *Graph
	g, err := Build(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	kinds := []string{KindRecursAs, KindHasDefect, KindYielded, KindConnectsTo, KindDependsOn, KindAffected, KindFixedBy, KindTrackedBy}
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

func TestNamedGraphValues(t *testing.T) {
	root := t.TempDir()
	writeInbox(t, root, "a.json", `{"id":"a","connects_to":["b"]}`)
	writeInbox(t, root, "b.json", `{"id":"b"}`)
	g, err := Build(root)
	if err != nil {
		t.Fatal(err)
	}

	var sub Subgraph
	sub, err = g.Neighbors("item:a", 1)
	if err != nil || len(sub.Nodes) != 2 || len(sub.Edges) != 1 || sub.Truncated {
		t.Fatalf("Neighbors = %+v, %v; want two nodes and one edge", sub, err)
	}
	var node Node = sub.Nodes[0]
	var edge Edge = sub.Edges[0]
	if node.ID == "" || node.Kind == "" || edge.From != "item:a" || edge.To != "item:b" || edge.Kind != KindConnectsTo {
		t.Errorf("node %+v / edge %+v do not carry the a->b connection", node, edge)
	}

	var rec Recurrence = g.Recurrences("no-such-fingerprint")
	if rec.Fingerprint != "no-such-fingerprint" || rec.Count != 0 || len(rec.Cycles) != 0 {
		t.Errorf("Recurrences of an unseen fingerprint = %+v, want zero", rec)
	}
}
