package carryover

import (
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

var cycleTokenRE = regexp.MustCompile(`(?i)\bcycle[ -]\d+`)

func fingerprint(action string) string {
	s := cycleTokenRE.ReplaceAllString(action, "cycle-N")
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

func fingerprintIndex(todos []cyclestate.CarryoverTodo, action string) int {
	fp := fingerprint(action)
	for i, t := range todos {
		if fingerprint(t.Action) == fp {
			return i
		}
	}
	return -1
}

func HasID(todos []cyclestate.CarryoverTodo, id string) bool {
	for _, t := range todos {
		if t.ID == id {
			return true
		}
	}
	return false
}
