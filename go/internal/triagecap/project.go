package triagecap

import "github.com/mickeyyaya/evolve-loop/go/internal/triagedecision"

func ProjectDecisionJSON(artifact string, cycle int) ([]byte, error) {
	return triagedecision.Project(artifact, cycle)
}
