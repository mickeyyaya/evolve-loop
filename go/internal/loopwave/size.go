package loopwave

import (
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleetbudget"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

// Size returns fc with Count sized for this wave, plus the inter-wave pace
// the loop should idle before the next wave (0 unless the budget is
// enforcing a floor-forced pace). Two composed layers: (1) the availability
// envelope — the Shrink port (fleet.QuotaAwareCount) shrinks Count per the
// active quota benches (min 1, one WARN per benched family on the warn
// writer); (2) the budget sizing — when the operator opted into a fleet.budget
// block, fleetbudget.Plan sizes the bench-shrunk count against the measured
// quota headroom + pace. Absent the block layer 2 is skipped entirely. With
// it, Stage governs application: "shadow" (the default) computes + LOGS the
// decision but HOLDS the count; "enforce" applies plan.Lanes and returns
// plan.PaceDelay. The [budget] report lines stay lines (INFO never reaches
// the console). Operates on a copy: benches expire and quota moves, so each
// wave re-reads them and the batch-level config never compounds a shrink.
func (e *Engine) Size(fc policy.FleetConfig, states []quotastate.QuotaState, tp budgethistory.Throughput, now time.Time) (policy.FleetConfig, time.Duration) {
	fc.Count = e.ports.Shrink(fc.Count, e.benchedFamilies(), fc.MinLanes, e.stderr)
	if fc.Budget == nil {
		return fc, 0
	}
	// S8 shadow join: a measurement, not a decision — logged on BOTH stages
	// before (and independently of) the sizing branch; never feeds the plan.
	if join, ok := fleetbudget.ShadowJoin(states, tp); ok {
		fmt.Fprintf(e.stderr, "[budget] shadow-join: %s\n", join.Reason)
	}
	plan := fleetbudget.Plan(states, tp, fleetbudget.Config{
		Count:          fc.Count,
		Floor:          fc.MinLanes,
		CapacityCycles: fc.Budget.CapacityCycles,
		Safety:         fc.Budget.Safety,
	}, now)
	switch fc.Budget.Stage {
	case "enforce":
		fmt.Fprintf(e.stderr, "[budget] enforce: sizing wave to %d lane(s) [%s] — %s\n", plan.Lanes, plan.DerivedFrom, plan.Reason)
		fc.Count = plan.Lanes
		return fc, plan.PaceDelay
	default:
		// shadow (the resolved default; any non-enforce stage): compute + log
		// the would-be decision, hold the count — a soak that never resizes.
		fmt.Fprintf(e.stderr, "[budget] shadow: would size wave to %d lane(s) (holding at %d) [%s] — %s\n", plan.Lanes, fc.Count, plan.DerivedFrom, plan.Reason)
		return fc, 0
	}
}

// benchedFamilies reads the quota-bench SSOT (clihealth.Store.Active()) and
// returns the active benches as family → reason for the wave's capacity
// shrink: lanes are full `evolve cycle run` subprocesses that may route to
// any installed family, so a benched family always shrinks the shared pool.
// Store.Active() degrades a missing/corrupt bench file to an empty map by
// design — bench state must never break a dispatch.
func (e *Engine) benchedFamilies() map[string]string {
	active := clihealth.NewStore(e.roots.ProjectRoot, nil).Active()
	if len(active) == 0 {
		return nil
	}
	benched := make(map[string]string, len(active))
	for fam, entry := range active {
		benched[fam] = entry.Reason
	}
	return benched
}
