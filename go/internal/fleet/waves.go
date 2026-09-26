package fleet

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/dag"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// PlanWaves levels todos by depends_on into waves of file-disjoint specs, merging file-sharing todos into one spec.
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
			specIDs := make([]string, len(group))
			optional := true
			for i, td := range group {
				specIDs[i] = td.ID
				if !td.Optional {
					optional = false
				}
			}
			specs = append(specs, CycleSpec{
				Scope:          specIDs,
				OutputContract: combinedContract(group),
				Env:            map[string]string{ipcenv.FleetScopeKey: strings.Join(specIDs, ",")},
				Optional:       optional,
			})
		}
		waves = append(waves, specs)
	}
	return waves, nil
}

// combinedContract keeps a lone todo's contract verbatim and labels each non-empty one "[id] " in a merged group.
func combinedContract(group []Todo) string {
	if len(group) == 1 {
		return strings.TrimSpace(group[0].OutputContract)
	}
	var parts []string
	for _, td := range group {
		if c := strings.TrimSpace(td.OutputContract); c != "" {
			parts = append(parts, "["+td.ID+"] "+c)
		}
	}
	return strings.Join(parts, "\n")
}

// groupByFiles union-finds todos that transitively share a file; groups follow their earliest member.
func groupByFiles(todos []Todo) [][]Todo {
	parent := make([]int, len(todos))
	for i := range parent {
		parent[i] = i
	}
	find := func(x int) int {
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
