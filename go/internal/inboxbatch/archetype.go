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
		tokens := declaredTokens([]string{f})
		if len(tokens) == 0 {
			return false
		}
		for _, tok := range tokens {
			if !strings.HasPrefix(tok, evolveStatePrefix) {
				return false
			}
		}
	}
	return true
}
