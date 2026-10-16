package recurrence

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
)

var genericPatternDenylist = map[string]bool{
	string(failurelog.OperatorReset):          true,
	string(failurelog.LoopFatal):              true,
	string(failurelog.UnknownClassification):  true,
	cyclestate.ClassificationMidExecutionFail: true,

	"cycle-fatal": true,
	"unknown":     true,
}

func IsGeneric(pattern, errorCategory string) bool {
	p := strings.TrimSpace(pattern)
	if p == "" {
		return true
	}
	if genericPatternDenylist[p] {
		return true
	}
	if ec := strings.TrimSpace(errorCategory); ec != "" && p == ec {
		return true
	}
	return false
}

func (l *Ledger) IsGenericPattern(pattern string) bool {
	if l == nil {
		return false
	}
	if e, ok := l.Entries[pattern]; ok {
		return e.Generic
	}
	return false
}
