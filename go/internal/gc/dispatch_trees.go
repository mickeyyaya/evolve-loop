package gc

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

const RuleStaleTree = "stale-tree"

func StaleTreeFiles(trees []proctree.TreeRecord, table []proctree.Process, o DispatchProcessOptions, planned []proctree.Process) []proctree.TreeRecord {
	reapable := proctree.Select(table, StaleDispatch(o))
	holding := proctree.Select(reapable, func(p proctree.Process) bool { return !proctree.Recorded(proctree.Identities(planned))(p) })
	var stale []proctree.TreeRecord
	for _, t := range trees {
		id, ok := proctree.ParseDispatchID(t.Dispatch)
		if !ok || o.PidAlive(id.Owner) || len(proctree.Select(holding, proctree.Recorded(t.Members))) > 0 {
			continue
		}
		stale = append(stale, t)
	}
	return stale
}
