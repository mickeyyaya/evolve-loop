package looppreflight

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

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

func defaultPhaseRoutingWarnings(projectRoot string) func() []string {
	return func() []string {
		_, _, warns, err := phasespec.MergedCatalog(projectRoot)
		if err != nil {
			return nil
		}
		return warns
	}
}
