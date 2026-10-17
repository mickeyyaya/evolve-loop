// Package kbgraph projects knowledge-base cycle dossiers and inbox items into a read-only in-memory graph.
package kbgraph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

const (
	MaxHops  = 2
	MaxNodes = 40

	KindRecursAs   = "recurs-as"
	KindHasDefect  = "has-defect"
	KindYielded    = "yielded"
	KindConnectsTo = "connects-to"
	KindDependsOn  = "depends-on"
)

type Node struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Label string `json:"label,omitempty"`
}

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type Subgraph struct {
	Nodes     []Node   `json:"nodes"`
	Edges     []Edge   `json:"edges"`
	Absent    []string `json:"absent"`
	Truncated bool     `json:"truncated"`
}

type Recurrence struct {
	Fingerprint string `json:"fingerprint"`
	Count       int    `json:"count"`
	Cycles      []int  `json:"cycles"`
}

type Graph struct {
	nodes map[string]Node
	edges []Edge
	adj   map[string][]string
	cycle map[int]string
}

func Build(projectRoot string) (*Graph, error) {
	g := &Graph{nodes: map[string]Node{}, adj: map[string][]string{}, cycle: map[int]string{}}
	for _, d := range dossier.ReadCommitted(projectRoot, 0) {
		g.addDossier(d)
	}
	if err := g.addInbox(filepath.Join(projectRoot, ".evolve", "inbox")); err != nil {
		return nil, err
	}
	for id := range g.adj {
		sort.Strings(g.adj[id])
	}
	sort.Slice(g.edges, func(i, j int) bool {
		a, b := g.edges[i], g.edges[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.To != b.To {
			return a.To < b.To
		}
		return a.Kind < b.Kind
	})
	return g, nil
}

func (g *Graph) addNode(id, kind, label string) {
	if _, ok := g.nodes[id]; !ok {
		g.nodes[id] = Node{ID: id, Kind: kind, Label: label}
	}
}

func (g *Graph) addEdge(from, to, kind string) {
	g.edges = append(g.edges, Edge{From: from, To: to, Kind: kind})
	g.adj[from] = append(g.adj[from], to)
	g.adj[to] = append(g.adj[to], from)
}

func (g *Graph) addDossier(d *dossier.Dossier) {
	id := fmt.Sprintf("cycle:%d", d.Cycle)
	g.addNode(id, "cycle", d.FinalVerdict)
	if d.Failure != nil && d.Failure.Fingerprint != "" {
		fp := "fingerprint:" + d.Failure.Fingerprint
		g.addNode(fp, "fingerprint", d.Failure.PreClass)
		g.addEdge(id, fp, KindRecursAs)
		g.cycle[d.Cycle] = d.Failure.Fingerprint
	}
	for _, def := range d.Defects {
		node := "defect:" + def.ID
		g.addNode(node, "defect", def.Severity)
		g.addEdge(id, node, KindHasDefect)
	}
	for _, l := range d.Lessons {
		node := "lesson:" + l.ID
		g.addNode(node, "lesson", l.Pattern)
		g.addEdge(id, node, KindYielded)
	}
}

func (g *Graph) addInbox(dir string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("kbgraph: read inbox %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		body, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("kbgraph: read %s: %w", path, err)
		}
		var item struct {
			ID         string   `json:"id"`
			Title      string   `json:"title"`
			ConnectsTo []string `json:"connects_to"`
			Deps       []string `json:"deps"`
		}
		if err := json.Unmarshal(body, &item); err != nil {
			return fmt.Errorf("kbgraph: parse %s: %w", path, err)
		}
		if item.ID == "" {
			continue
		}
		from := "item:" + item.ID
		g.addNode(from, "item", item.Title)
		for _, to := range item.ConnectsTo {
			g.addNode("item:"+to, "item", "")
			g.addEdge(from, "item:"+to, KindConnectsTo)
		}
		for _, to := range item.Deps {
			g.addNode("item:"+to, "item", "")
			g.addEdge(from, "item:"+to, KindDependsOn)
		}
	}
	return nil
}

func (g *Graph) Neighbors(id string, hops int) (Subgraph, error) {
	if _, ok := g.nodes[id]; !ok {
		return Subgraph{}, fmt.Errorf("kbgraph: unknown node %q", id)
	}
	hops = max(1, min(hops, MaxHops))
	included := map[string]bool{id: true}
	order := []string{id}
	truncated := false
	frontier := []string{id}
	for depth := 0; depth < hops; depth++ {
		var next []string
		for _, cur := range frontier {
			for _, n := range g.adj[cur] {
				if included[n] {
					continue
				}
				if len(order) >= MaxNodes {
					truncated = true
					continue
				}
				included[n] = true
				order = append(order, n)
				next = append(next, n)
			}
		}
		frontier = next
	}
	sub := Subgraph{Nodes: []Node{}, Edges: []Edge{}, Absent: g.absent(order), Truncated: truncated}
	for _, n := range order {
		sub.Nodes = append(sub.Nodes, g.nodes[n])
	}
	for _, e := range g.edges {
		if included[e.From] && included[e.To] {
			sub.Edges = append(sub.Edges, e)
		}
	}
	return sub, nil
}

func (g *Graph) absent(ids []string) []string {
	missing := map[string]bool{}
	for _, id := range ids {
		if g.nodes[id].Kind != "cycle" {
			continue
		}
		have := map[string]bool{}
		for _, e := range g.edges {
			if e.From == id {
				have[e.Kind] = true
			}
		}
		for _, k := range []string{KindRecursAs, KindHasDefect, KindYielded} {
			if !have[k] {
				missing[k] = true
			}
		}
	}
	out := []string{}
	for k := range missing {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (g *Graph) Recurrences(fingerprint string) Recurrence {
	rec := Recurrence{Fingerprint: fingerprint, Cycles: []int{}}
	for c, fp := range g.cycle {
		if fp == fingerprint {
			rec.Cycles = append(rec.Cycles, c)
		}
	}
	sort.Ints(rec.Cycles)
	rec.Count = len(rec.Cycles)
	return rec
}
