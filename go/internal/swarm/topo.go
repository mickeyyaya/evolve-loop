package swarm

import "github.com/mickeyyaya/evolve-loop/go/internal/dag"

// TopoOrder returns the writers' merge order over the depends_on DAG, ties broken by worker_id; an unknown dependency or a cycle is an error.
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
