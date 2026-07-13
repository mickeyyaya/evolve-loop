package runner

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// LaneScope resolves the fleet lane scope pinned for this run (ADR-0049 E;
// cycle-766 lane-scope.json → Context["fleet_scope"]): the typed envelope at
// enforce, the legacy Context map below it (byte-identical — Active() is
// false unless enforce). The ids come from the advisor's (LLM-authored)
// backlog, so control chars are collapsed before the value reaches a prompt —
// a newline in an id would otherwise forge a new context bullet (prompt
// injection via the data channel). Empty string ⇒ not a fleet lane; phases
// must render nothing (sequential cycles stay byte-identical).
func LaneScope(req core.PhaseRequest) string {
	scope := req.Context["fleet_scope"]
	if req.Input.Active() {
		scope = req.Input.CycleInputs().FleetScope()
	}
	return sanitizeLaneScopeValue(scope)
}

// sanitizeLaneScopeValue collapses newlines, carriage returns, and tabs to
// spaces so advisor-authored data injected into a prompt context bullet
// cannot forge a new line / directive.
func sanitizeLaneScopeValue(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, s)
}
