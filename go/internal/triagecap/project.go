package triagecap

import "github.com/mickeyyaya/evolve-loop/go/internal/triagedecision"

// ProjectDecisionJSON derives triage-decision.json from a triage report; a missing section projects as [].
func ProjectDecisionJSON(artifact string, cycle int) ([]byte, error) {
	return triagedecision.Project(artifact, cycle)
}
