package looppreflight

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

// checkCLIHealth warns at batch start that benched CLI families will run fallback-first.
// Never a halt: the fallback chain exists so a benched family does not block the batch.
func checkCLIHealth(o resolved) CheckResult {
	const name = "cli-health"
	active := o.cliHealthActive()
	if len(active) == 0 {
		return CheckResult{Name: name, Level: LevelPass,
			Message: "no active CLI-family benches"}
	}
	lines := make([]string, 0, len(active))
	for _, e := range active {
		lines = append(lines, fmt.Sprintf("%s benched until %s (%s, strikes=%d) — dispatch chains start at fallback",
			e.Family, e.BenchedUntil.Format("15:04 MST"), e.Reason, e.Strikes))
	}
	sort.Strings(lines)
	return CheckResult{Name: name, Level: LevelWarn,
		Message: fmt.Sprintf("%d CLI family/families benched (transient wall remembered)", len(active)),
		Detail:  strings.Join(lines, "\n"),
	}
}

func defaultCLIHealthActive(projectRoot string) func() []clihealth.Entry {
	return func() []clihealth.Entry {
		active := clihealth.NewStore(projectRoot, nil).Active()
		out := make([]clihealth.Entry, 0, len(active))
		for _, e := range active {
			out = append(out, e)
		}
		return out
	}
}
