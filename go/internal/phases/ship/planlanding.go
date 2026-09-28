package ship

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// PlanLanding routes fleet lanes to a landing plan for the fleet main-push path,
// per cfg.Landing.
func PlanLanding(cfg policy.FleetConfig, lanes []fleet.LaneCandidate) [][]string {
	if len(lanes) == 0 {
		return nil
	}
	if cfg.Landing == "prefix-queue" {
		q := fleet.NewPrefixQueue()
		for _, l := range lanes {
			q.Enqueue(l)
		}
		return q.ComposePrefixes()
	}
	plan := make([][]string, 0, len(lanes))
	for _, l := range lanes {
		plan = append(plan, []string{l.ID})
	}
	return plan
}
