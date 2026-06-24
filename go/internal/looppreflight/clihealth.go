package looppreflight

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

// checkCLIHealth surfaces ACTIVE CLI-family benches (.evolve/cli-health.json,
// written when a dispatch died on a classified transient wall like
// rate_limit) so the operator sees AT BATCH START that chains will run
// fallback-first — instead of discovering it from per-phase fallback logs
// (cycle-283: codex was quota-walled all night and only the dispatch trail
// showed it). Always Warn, never Halt: the fallback chain exists precisely so
// a benched family doesn't block the batch, and expired benches are canaried
// per-cycle by the loop.
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

// defaultCLIHealthActive reads the real bench store.
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
