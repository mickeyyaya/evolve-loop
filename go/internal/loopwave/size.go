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

func (e *Engine) Size(fc policy.FleetConfig, states []quotastate.QuotaState, tp budgethistory.Throughput, now time.Time) (policy.FleetConfig, time.Duration) {
	fc.Count = e.ports.Shrink(fc.Count, e.benchedFamilies(), fc.MinLanes, e.stderr)
	if fc.Budget == nil {
		return fc, 0
	}
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
		fmt.Fprintf(e.stderr, "[budget] shadow: would size wave to %d lane(s) (holding at %d) [%s] — %s\n", plan.Lanes, fc.Count, plan.DerivedFrom, plan.Reason)
		return fc, 0
	}
}

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
