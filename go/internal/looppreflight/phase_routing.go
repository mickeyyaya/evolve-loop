package looppreflight

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// checkPhaseRoutingWarnings surfaces user-phase specs that phasespec DROPPED
// during catalog merge/routing (a built-in-name hijack, a non-optional
// override, a malformed phase.json) into the SAME accumulated, gate-visible
// preflight Result every other readiness problem lands in. Before this check,
// those warnings were only fmt.Fprintf'd to stderr by the CLI/dispatch callers
// and discarded — an operator who typo'd a user phase.json got a silently
// missing phase and no batch-start signal (scout cycle-591 Beyond-the-Ask).
//
// Always Warn, never Halt: a dropped user phase is degraded-but-runnable — the
// built-in spine is untouched, so the batch must still start. Routing a
// legitimate memo-overlay typo straight to Halt would turn every working
// deployment into a batch-blocker the moment any unrelated phase.json breaks.
func checkPhaseRoutingWarnings(o resolved) CheckResult {
	const name = "phase-routing-warnings"
	warns := o.phaseRoutingWarnings()
	if len(warns) == 0 {
		return CheckResult{Name: name, Level: LevelPass,
			Message: "no dropped/invalid user-phase routing specs"}
	}
	return CheckResult{Name: name, Level: LevelWarn,
		Message: fmt.Sprintf("%d user-phase routing spec(s) dropped (built-in spine intact)", len(warns)),
		Detail:  strings.Join(warns, "\n"),
	}
}

// defaultPhaseRoutingWarnings wires the production default to the real merged
// catalog. A catalog-load error is SWALLOWED (fail-open, mirroring
// DiscoverUserSpecs' "missing dir → no specs" posture): a readiness gate must
// never itself become the reason a batch can't start.
func defaultPhaseRoutingWarnings(projectRoot string) func() []string {
	return func() []string {
		_, _, warns, err := phasespec.MergedCatalog(projectRoot)
		if err != nil {
			return nil
		}
		return warns
	}
}
