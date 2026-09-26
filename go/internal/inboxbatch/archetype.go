package inboxbatch

import "strings"

const (
	operatorStateClass = "pipeline-architecture"
	// evolveStatePrefix keeps its trailing "/" so ".evolvex/" is not runtime state.
	evolveStatePrefix = ".evolve/"
)

// IsOperatorState reports whether a pipeline-architecture item touches only .evolve/ runtime state.
// It answers false on short evidence: a false positive would skip review of a source change.
func IsOperatorState(it Item) bool {
	if it.Class != operatorStateClass || len(it.Files) == 0 {
		return false
	}
	for _, f := range it.Files {
		if !strings.HasPrefix(f, evolveStatePrefix) {
			return false
		}
	}
	return true
}
