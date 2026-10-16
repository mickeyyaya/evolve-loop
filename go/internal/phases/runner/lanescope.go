package runner

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func LaneScope(req core.PhaseRequest) string {
	scope := req.Context["fleet_scope"]
	if req.Input.Active() {
		scope = req.Input.CycleInputs().FleetScope()
	}
	return sanitizeLaneScopeValue(scope)
}

func sanitizeLaneScopeValue(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, s)
}
