package looppreflight

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// checkPhaseRoutingWarnings surfaces user-phase specs phasespec dropped during catalog merge.
// Never a halt: the built-in spine is intact, so a broken phase.json must not block the batch.
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

// defaultPhaseRoutingWarnings fails open on a catalog-load error, so the gate never blocks a batch by itself.
func defaultPhaseRoutingWarnings(projectRoot string) func() []string {
	return func() []string {
		_, _, warns, err := phasespec.MergedCatalog(projectRoot)
		if err != nil {
			return nil
		}
		return warns
	}
}
