package swarm

import (
	"fmt"
	"sort"
)

// TopoOrder returns the serialized merge-train order for a writer swarm:
// Kahn's algorithm over the inter-worker depends_on DAG, so a worker merges into
// the integration branch only after every worker it depends on is already
// merged. Ties (workers with equal in-degree, ready at the same step) are broken
// by worker_id ascending so the order is deterministic across runs.
//
// It is a pure function over the worker specs — no I/O — and returns an error if
// depends_on references an unknown worker or forms a cycle (either makes a
// serialized merge order impossible; the validator surfaces it as a collapse to
// N=1).
func TopoOrder(workers []WorkerSpec) ([]string, error) {
	indegree := make(map[string]int, len(workers))
	deps := make(map[string][]string, len(workers))
	known := make(map[string]bool, len(workers))
	for _, w := range workers {
		known[w.WorkerID] = true
		indegree[w.WorkerID] = 0 // ensure isolated workers (no deps) still appear
	}
	for _, w := range workers {
		for _, d := range w.DependsOn {
			if !known[d] {
				return nil, fmt.Errorf("worker %q depends_on unknown worker %q", w.WorkerID, d)
			}
			if d == w.WorkerID {
				return nil, fmt.Errorf("worker %q depends_on itself", w.WorkerID)
			}
			deps[d] = append(deps[d], w.WorkerID)
			indegree[w.WorkerID]++
		}
	}

	// Ready set: all in-degree-0 workers, processed in worker_id order.
	var ready []string
	for id, deg := range indegree {
		if deg == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)

	order := make([]string, 0, len(workers))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		order = append(order, id)
		for _, dependent := range deps[id] {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				ready = append(ready, dependent)
			}
		}
		sort.Strings(ready) // keep tie-break deterministic
	}

	if len(order) != len(indegree) {
		return nil, fmt.Errorf("depends_on graph has a cycle (ordered %d of %d workers)", len(order), len(indegree))
	}
	return order, nil
}
