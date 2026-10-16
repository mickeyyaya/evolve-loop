package inboxbatch

import "strings"

const (
	operatorStateClass = "pipeline-architecture"
	evolveStatePrefix  = ".evolve/"
)

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
