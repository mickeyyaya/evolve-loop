// Package dag provides deterministic dependency-graph leveling (Kahn's
// algorithm by level).
package dag

import (
	"fmt"
	"sort"
)

func Levels(nodes []string, deps map[string][]string) ([][]string, error) {
	indegree, dependents, err := dependencyEdges(nodes, deps)
	if err != nil {
		return nil, err
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

func dependencyEdges(nodes []string, deps map[string][]string) (map[string]int, map[string][]string, error) {
	known := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		known[n] = true
	}
	indegree := make(map[string]int, len(nodes))
	dependents := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		indegree[n] = 0
	}
	for node, ds := range deps {
		if !known[node] {
			return nil, nil, fmt.Errorf("dag: dependency key %q is not a known node", node)
		}
		for _, d := range ds {
			if !known[d] {
				return nil, nil, fmt.Errorf("dag: node %q depends on unknown node %q", node, d)
			}
			if d == node {
				return nil, nil, fmt.Errorf("dag: node %q depends on itself", node)
			}
			dependents[d] = append(dependents[d], node)
			indegree[node]++
		}
	}
	return indegree, dependents, nil
}

func Flatten(levels [][]string) []string {
	var out []string
	for _, lvl := range levels {
		out = append(out, lvl...)
	}
	return out
}
