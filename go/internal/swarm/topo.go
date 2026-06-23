package swarm

import "github.com/mickeyyaya/evolveloop/go/internal/dag"

// TopoOrder returns the serialized merge-train order for a writer swarm: the
// flattened topological order over the inter-worker depends_on DAG, so a worker
// merges into the integration branch only after every worker it depends on is
// already merged. Ties (workers ready at the same level) break by worker_id
// ascending so the order is deterministic across runs.
//
// It is a pure function over the worker specs — no I/O — and returns an error if
// depends_on references an unknown worker or forms a cycle (either makes a
// serialized merge order impossible; the validator surfaces it as a collapse to
// N=1). The topological logic lives in internal/dag, shared with the campaign
// wave planner so Kahn's algorithm is not duplicated.
func TopoOrder(workers []WorkerSpec) ([]string, error) {
	nodes := make([]string, len(workers))
	deps := make(map[string][]string, len(workers))
	for i, w := range workers {
		nodes[i] = w.WorkerID
		if len(w.DependsOn) > 0 {
			deps[w.WorkerID] = w.DependsOn
		}
	}
	levels, err := dag.Levels(nodes, deps)
	if err != nil {
		return nil, err
	}
	return dag.Flatten(levels), nil
}
