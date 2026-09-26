package runner

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// LaneScope returns the fleet lane scope pinned for this run, sanitized for a prompt; empty means not a fleet lane.
func LaneScope(req core.PhaseRequest) string {
	scope := req.Context["fleet_scope"]
	if req.Input.Active() {
		scope = req.Input.CycleInputs().FleetScope()
	}
	return sanitizeLaneScopeValue(scope)
}

// sanitizeLaneScopeValue collapses line breaks and tabs, so LLM-authored ids cannot forge a new prompt line.
func sanitizeLaneScopeValue(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, s)
}
