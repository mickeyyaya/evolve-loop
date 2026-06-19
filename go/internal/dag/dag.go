// Package dag provides deterministic dependency-graph leveling (Kahn's algorithm
// by level). It is the single home for the topological logic shared by the
// campaign wave planner (which needs concurrent waves) and the swarm merge-train
// ordering (which needs the flattened serial order) — so Kahn's lives in exactly
// one place.
package dag

import (
	"fmt"
	"sort"
)

// Levels groups nodes into dependency waves: level 0 holds every node with no
// dependencies; level k holds nodes whose dependencies all resolve in levels < k.
// Nodes within a level have no inter-dependency and may run concurrently. Order
// within a level is ascending so the result is deterministic across runs.
//
// deps maps a node to the nodes it depends on (which must run first). Levels
// returns an error if a dependency references an unknown node (dangling ref), a
// dependency key is not a known node, a node depends on itself, or the graph
// contains a cycle — each makes a valid leveling impossible.
func Levels(nodes []string, deps map[string][]string) ([][]string, error) {
	known := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		known[n] = true
	}
	indegree := make(map[string]int, len(nodes))
	dependents := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		indegree[n] = 0 // isolated nodes (no deps) still appear
	}
	for node, ds := range deps {
		if !known[node] {
			return nil, fmt.Errorf("dag: dependency key %q is not a known node", node)
		}
		for _, d := range ds {
			if !known[d] {
				return nil, fmt.Errorf("dag: node %q depends on unknown node %q", node, d)
			}
			if d == node {
				return nil, fmt.Errorf("dag: node %q depends on itself", node)
			}
			dependents[d] = append(dependents[d], node)
			indegree[node]++
		}
	}

	var ready []string
	for n, deg := range indegree {
		if deg == 0 {
			ready = append(ready, n)
		}
	}

	var levels [][]string
	placed := 0
	for len(ready) > 0 {
		sort.Strings(ready)
		level := ready
		levels = append(levels, level)
		placed += len(level)
		var next []string
		for _, n := range level {
			for _, dep := range dependents[n] {
				indegree[dep]--
				if indegree[dep] == 0 {
					next = append(next, dep)
				}
			}
		}
		ready = next
	}

	if placed != len(indegree) {
		return nil, fmt.Errorf("dag: graph has a cycle (%d of %d nodes leveled)", placed, len(indegree))
	}
	return levels, nil
}

// Flatten concatenates levels into a single serial topological order (level order
// preserved, ascending within each level).
func Flatten(levels [][]string) []string {
	var out []string
	for _, lvl := range levels {
		out = append(out, lvl...)
	}
	return out
}
