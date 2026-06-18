package fleet

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/dag"
)

// PlanWaves turns a dependency-annotated backlog into an ordered list of waves.
// Wave k holds cycle specs whose todos depend only on todos in waves < k (the
// depends_on DAG, leveled via internal/dag). Within a wave, todos are grouped
// into FILE-DISJOINT cycles: todos that (transitively) share a file cluster into
// one cycle that runs them sequentially in a single worktree — safe — while
// distinct cycles in a wave touch disjoint files and run concurrently. This is
// the campaign analogue of fleet.Partition, but it MERGES file-sharing todos
// (correct within one wave) rather than deferring them to a later wave.
//
// It returns an error if the depends_on graph is cyclic or references an unknown
// todo (dag.Levels surfaces both) — a campaign that can't be ordered must not run.
func PlanWaves(todos []Todo) ([][]CycleSpec, error) {
	ids := make([]string, len(todos))
	byID := make(map[string]Todo, len(todos))
	deps := make(map[string][]string, len(todos))
	for i, td := range todos {
		ids[i] = td.ID
		byID[td.ID] = td
		if len(td.DependsOn) > 0 {
			deps[td.ID] = td.DependsOn
		}
	}
	levels, err := dag.Levels(ids, deps)
	if err != nil {
		return nil, err
	}
	waves := make([][]CycleSpec, 0, len(levels))
	for _, level := range levels {
		levelTodos := make([]Todo, 0, len(level))
		for _, id := range level {
			levelTodos = append(levelTodos, byID[id])
		}
		var specs []CycleSpec
		for _, group := range groupByFiles(levelTodos) {
			ids := make([]string, len(group))
			for i, td := range group {
				ids[i] = td.ID
			}
			specs = append(specs, CycleSpec{
				Scope: ids,
				Env:   map[string]string{fleetScopeEnvKey: strings.Join(ids, ",")},
			})
		}
		waves = append(waves, specs)
	}
	return waves, nil
}

// groupByFiles partitions todos into file-disjoint groups: two todos share a
// group iff they (transitively) share a normalized file. Each group becomes one
// cycle (its todos run sequentially in one worktree — file-safe); distinct groups
// touch disjoint files and may run concurrently. Union-find over the
// shares-a-file relation. Deterministic: groups are ordered by their
// earliest-appearing member, and todos within a group keep input order.
func groupByFiles(todos []Todo) [][]Todo {
	parent := make([]int, len(todos))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]] // path-halving
			x = parent[x]
		}
		return x
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra == rb {
			return
		}
		if ra < rb { // root = smallest member index → deterministic
			parent[rb] = ra
		} else {
			parent[ra] = rb
		}
	}
	fileOwner := map[string]int{} // normalized file -> first todo index claiming it
	for i, td := range todos {
		for f := range normalizeFiles(td.Files) {
			if j, ok := fileOwner[f]; ok {
				union(i, j)
			} else {
				fileOwner[f] = i
			}
		}
	}
	var order []int
	groups := map[int][]Todo{}
	for i, td := range todos {
		r := find(i)
		if _, seen := groups[r]; !seen {
			order = append(order, r)
		}
		groups[r] = append(groups[r], td)
	}
	out := make([][]Todo, 0, len(order))
	for _, r := range order {
		out = append(out, groups[r])
	}
	return out
}
